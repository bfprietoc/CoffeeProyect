package service

import (
	"encoding/json"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/payment"
	"coffeeproyect/internal/store"
	"fmt"
	"log"
	"strings"
	"time"
)

// PaymentConfig carries the secrets/config Wompi needs. Missing values make
// the feature disabled (503), so the app never crashes or half-works.
type PaymentConfig struct {
	PublicKey       string
	IntegritySecret string
	EventSecret     string
	BaseURL         string // PUBLIC_BASE_URL, e.g. https://shop.example.com
}

// PaymentService builds Web Checkout params and processes Wompi webhooks.
type PaymentService struct {
	orders store.OrderStore
	users  store.UserStore
	cfg    PaymentConfig
	now    func() time.Time
}

func NewPaymentService(orders store.OrderStore, users store.UserStore, cfg PaymentConfig) *PaymentService {
	return &PaymentService{
		orders: orders,
		users:  users,
		cfg:    cfg,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// Enabled reports whether the payment integration is configured.
func (s *PaymentService) Enabled() bool {
	return s.cfg.PublicKey != "" && s.cfg.IntegritySecret != "" && s.cfg.EventSecret != ""
}

// CreateCheckout validates the order and returns the Web Checkout params for
// a new payment attempt. The attempt gets a unique reference
// (order.id + unix timestamp) so retries never collide on Wompi's side
// (422 duplicate reference).
func (s *PaymentService) CreateCheckout(orderID, userID string) (payment.CheckoutParams, error) {
	if !s.Enabled() {
		return payment.CheckoutParams{}, domain.ErrPaymentDisabled
	}

	order, err := s.orders.GetByIDAndUser(orderID, userID)
	if err != nil {
		return payment.CheckoutParams{}, err
	}
	if order.Status == domain.OrderStatusCancelled {
		return payment.CheckoutParams{}, domain.ErrPaymentUnavailable
	}
	if order.PaymentStatus == domain.PaymentStatusPaid {
		return payment.CheckoutParams{}, domain.ErrPaymentAlreadyPaid
	}

	user, err := s.users.GetByID(userID)
	if err != nil {
		return payment.CheckoutParams{}, err
	}

	reference := fmt.Sprintf("%s-%d", order.ID, s.now().Unix())
	provider := payment.ProviderWompi
	newOrder, err := s.orders.UpdatePayment(orderID, domain.PaymentPatch{
		Reference: &reference,
		SetStatus: true,
		Status:    domain.PaymentStatusPending,
		Provider:  &provider,
		SetPaidAt: true,
		PaidAt:    nil, // new attempt: no paid timestamp yet
	})
	if err != nil {
		return payment.CheckoutParams{}, err
	}

	// BaseURL is optional: when empty the checkout form omits the redirect
	// URL and Wompi shows its own success page (the webhook is the source of
	// truth regardless). When set, the customer lands back on our order.
	redirectURL := ""
	if base := strings.TrimSuffix(s.cfg.BaseURL, "/"); base != "" {
		redirectURL = fmt.Sprintf("%s/orders/%s?payment=1", base, newOrder.ID)
	}
	return payment.CheckoutParams{
		PublicKey:     s.cfg.PublicKey,
		Reference:     reference,
		AmountInCents: newOrder.TotalCents,
		Currency:      newOrder.Currency,
		Signature:     payment.IntegritySignature(reference, newOrder.TotalCents, newOrder.Currency, s.cfg.IntegritySecret),
		RedirectURL:   redirectURL,
		CustomerEmail: user.Email,
	}, nil
}

// HandleWebhook validates and processes a Wompi webhook event.
// Unknown references and unrecognized events are logged and ignored (200) —
// they are not errors worth retrying (Wompi retries on non-200 at most 3
// times in 24h and we never want to loop on data we can't map).
func (s *PaymentService) HandleWebhook(rawBody []byte) error {
	if !s.Enabled() {
		return domain.ErrPaymentDisabled
	}

	evt, err := payment.VerifyAndParseEvent(rawBody, s.cfg.EventSecret)
	if err != nil {
		// Debug: log the exact failure (never the secret) to diagnose silent
		// 401s from the Wompi dashboard.
		var evtInfo map[string]any
		_ = json.Unmarshal(rawBody, &evtInfo)
		props := ""
		if sig, ok := evtInfo["signature"].(map[string]any); ok {
			if list, ok := sig["properties"].([]any); ok {
				strs := make([]string, 0, len(list))
				for _, it := range list {
					if s, ok := it.(string); ok {
						strs = append(strs, s)
					}
				}
				props = strings.Join(strs, ",")
			}
		}
		ts := ""
		if t, ok := evtInfo["timestamp"]; ok {
			ts = fmt.Sprintf("%v", t)
		}
		log.Printf("wompi webhook: verification failed: %v (event=%s properties=%s timestamp=%s)", err, evtInfo["event"], props, ts)
		return fmt.Errorf("webhook signature: %w", err)
	}

	tx, ok := payment.TransactionFromEvent(evt)
	if !ok {
		log.Printf("wompi webhook: ignoring unrecognized event %q", evt.Event)
		return nil
	}
	if tx.Reference == "" {
		log.Printf("wompi webhook: transaction %s has no reference, ignoring", tx.ID)
		return nil
	}

	order, err := s.orders.GetByPaymentReference(tx.Reference)
	if err != nil {
		// The current reference may have been superseded by a retry, or the
		// reference is unknown. Never trust the redirect; the webhook is the
		// source of truth. Log loudly so a double payment can be seen.
		log.Printf("wompi webhook: order for reference %s not found (transaction %s, status %s) — ignoring",
			tx.Reference, tx.ID, tx.Status)
		return nil
	}

	status, terminal := payment.FinalStatus(tx.Status)
	if !terminal {
		// Still processing — no state change, just 200.
		return nil
	}
	// Idempotency: Wompi retries non-200s and the order row already
	// reflects the event. Nothing to do.
	if order.PaymentStatus == domain.PaymentStatusPaid && status == "paid" {
		return nil
	}
	if order.PaymentStatus == domain.PaymentStatusFailed && status == "failed" {
		return nil
	}
	// Final states only. The row below is updated against the CURRENT
	// reference (the event came from the matching attempt).
	paidAt := s.now()
	seen := tx.ID
	switch status {
	case "paid":
		_, err = s.orders.UpdatePayment(order.ID, domain.PaymentPatch{
			SetStatus:     true,
			Status:        domain.PaymentStatusPaid,
			SetPaidAt:     true,
			PaidAt:        &paidAt,
			TransactionID: &seen,
		})
	case "failed":
		_, err = s.orders.UpdatePayment(order.ID, domain.PaymentPatch{
			SetStatus:     true,
			Status:        domain.PaymentStatusFailed,
			SetPaidAt:     true,
			PaidAt:        nil,
			TransactionID: &seen,
		})
	default:
		return nil
	}
	if err != nil {
		return fmt.Errorf("apply webhook: %w", err)
	}
	log.Printf("wompi webhook: order %s payment_status=%s (transaction %s)", order.ID, status, tx.ID)
	return nil
}

// PaymentEnabled reports whether the given config is fully populated.
// Used by main.go to log a startup warning (the service also re-checks on
// every request).
func PaymentEnabled(cfg PaymentConfig) bool {
	return cfg.PublicKey != "" && cfg.IntegritySecret != "" && cfg.EventSecret != ""
}
