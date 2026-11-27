package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/payment"
	memstore "coffeeproyect/internal/store/memory"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func testPaymentConfig() PaymentConfig {
	return PaymentConfig{
		PublicKey:       "pub_test_xxx",
		IntegritySecret: "prod_integrity_test",
		EventSecret:     "prod_events_test",
		BaseURL:         "http://localhost:3001",
	}
}

// setupPaymentService returns the service, its stores and the real IDs.
func setupPaymentService(t *testing.T) (*PaymentService, *memstore.OrderStore, string, string) {
	t.Helper()
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()
	user, err := userStore.Create(domain.User{Email: "test@example.com", Name: "Test"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	order := domain.Order{
		UserID:          user.ID,
		Status:          domain.OrderStatusConfirmed,
		TotalCents:      8500000,
		Currency:        "COP",
		ShippingStreet:  "Calle 1",
		ShippingCity:    "Bogotá",
		ShippingDept:    "Cundinamarca",
		ShippingCountry: "CO",
		Items: []domain.OrderItem{
			{CoffeeID: availCoffeeID, CoffeeName: "El Paraíso 92", Quantity: 1, UnitPriceCents: 8500000},
		},
	}
	created, err := orderStore.Create(order)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	svc := NewPaymentService(orderStore, userStore, testPaymentConfig())
	return svc, orderStore, created.ID, user.ID
}

func makeWebhookEvent(t *testing.T, secret string, properties []string, data map[string]any, ts int64) []byte {
	t.Helper()
	chain := ""
	for _, prop := range properties {
		parts := strings.Split(prop, ".")
		var cur any = data
		for _, p := range parts {
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("property %q not in data", prop)
			}
			cur, ok = m[p]
			if !ok {
				t.Fatalf("property %q not in data", prop)
			}
		}
		switch v := cur.(type) {
		case string:
			chain += v
		case float64:
			if v == float64(int64(v)) {
				chain += fmt.Sprintf("%d", int64(v))
			} else {
				chain += fmt.Sprintf("%v", v)
			}
		default:
			chain += fmt.Sprintf("%v", v)
		}
	}
	chain += fmt.Sprintf("%d", ts)
	chain += secret
	sum := sha256.Sum256([]byte(chain))
	checksum := hex.EncodeToString(sum[:])

	evt := map[string]any{
		"event":       "transaction.updated",
		"environment": "test",
		"timestamp":   ts,
		"data":        data,
		"signature":   map[string]any{"properties": properties, "checksum": checksum},
	}
	raw, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return raw
}

func checkoutParams(t *testing.T, svc *PaymentService, store *memstore.OrderStore, orderID, userID string) payment.CheckoutParams {
	t.Helper()
	params, err := svc.CreateCheckout(orderID, userID)
	if err != nil {
		t.Fatalf("create checkout: %v", err)
	}
	return params
}

func TestCreateCheckout_disabledNotConfigured(t *testing.T) {
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()
	svc := NewPaymentService(orderStore, userStore, PaymentConfig{})
	_, err := svc.CreateCheckout("some-id", "some-user")
	if err != domain.ErrPaymentDisabled {
		t.Errorf("expected ErrPaymentDisabled, got %v", err)
	}
}

func TestCreateCheckout_notFound(t *testing.T) {
	svc, _, _, userID := setupPaymentService(t)
	_, err := svc.CreateCheckout("no-such-order", userID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateCheckout_wrongUser(t *testing.T) {
	svc, _, orderID, _ := setupPaymentService(t)
	_, err := svc.CreateCheckout(orderID, "other-user")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateCheckout_success(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	params, err := svc.CreateCheckout(orderID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.PublicKey != "pub_test_xxx" {
		t.Errorf("wrong public key: %s", params.PublicKey)
	}
	if params.AmountInCents != 8500000 {
		t.Errorf("wrong amount: %d", params.AmountInCents)
	}
	if params.Currency != "COP" {
		t.Errorf("wrong currency: %s", params.Currency)
	}
	if params.Reference == "" || !strings.HasPrefix(params.Reference, orderID) {
		t.Errorf("reference should be %s-<timestamp>, got %q", orderID, params.Reference)
	}
	if params.Signature == "" {
		t.Error("signature must not be empty")
	}
	if !strings.Contains(params.RedirectURL, "/orders/"+orderID+"?payment=1") {
		t.Errorf("redirect URL must point at the order detail page: %s", params.RedirectURL)
	}
	if params.CustomerEmail != "test@example.com" {
		t.Errorf("customer email should come from the profile: %s", params.CustomerEmail)
	}

	// The order must carry the pending status + reference + provider.
	o, err := store.GetByID(orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if o.PaymentStatus != domain.PaymentStatusPending {
		t.Errorf("expected pending, got %s", o.PaymentStatus)
	}
	if o.PaymentReference == nil || *o.PaymentReference != params.Reference {
		t.Errorf("stored reference does not match checkout reference: %v", o.PaymentReference)
	}
	if o.PaymentProvider == nil || *o.PaymentProvider != "wompi" {
		t.Errorf("provider must be wompi: %v", o.PaymentProvider)
	}
	if o.PaidAt != nil {
		t.Error("paid_at must be nil for a new attempt")
	}
}

func TestCreateCheckout_signatureIsIntegrityHash(t *testing.T) {
	svc, _, orderID, userID := setupPaymentService(t)
	params, err := svc.CreateCheckout(orderID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := payment.IntegritySignature(params.Reference, 8500000, "COP", "prod_integrity_test")
	if params.Signature != want {
		t.Errorf("signature must use the integrity secret: got %s, want %s", params.Signature, want)
	}
}

func TestCreateCheckout_alreadyPaid(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	paidAt := time.Now().UTC()
	_, err := store.UpdatePayment(orderID, domain.PaymentPatch{
		SetStatus: true, Status: domain.PaymentStatusPaid, SetPaidAt: true, PaidAt: &paidAt,
	})
	if err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	_, err = svc.CreateCheckout(orderID, userID)
	if err != domain.ErrPaymentAlreadyPaid {
		t.Errorf("expected ErrPaymentAlreadyPaid, got %v", err)
	}
}

func TestCreateCheckout_cancelledOrder(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	_, err := store.Cancel(orderID, userID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	_, err = svc.CreateCheckout(orderID, userID)
	if err != domain.ErrPaymentUnavailable {
		t.Errorf("expected ErrPaymentUnavailable, got %v", err)
	}
}

func TestHandleWebhook_approved(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	params := checkoutParams(t, svc, store, orderID, userID)

	data := map[string]any{
		"transaction": map[string]any{
			"id":                  "1234-1610641025-49201",
			"reference":           params.Reference,
			"status":              "APPROVED",
			"amount_in_cents":     8500000,
			"currency":            "COP",
			"payment_method_type": "CARD",
		},
	}
	raw := makeWebhookEvent(t, "prod_events_test", []string{"transaction.id", "transaction.status", "transaction.amount_in_cents"}, data, 1530291411)

	if err := svc.HandleWebhook(raw); err != nil {
		t.Fatalf("handle webhook: %v", err)
	}

	o, err := store.GetByID(orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if o.PaymentStatus != domain.PaymentStatusPaid {
		t.Errorf("expected paid, got %s", o.PaymentStatus)
	}
	if o.PaidAt == nil {
		t.Error("paid_at must be set after approval")
	}
	if o.PaymentTransactionID == nil || *o.PaymentTransactionID != "1234-1610641025-49201" {
		t.Errorf("transaction id not stored: %v", o.PaymentTransactionID)
	}
}

func TestHandleWebhook_declined(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	params := checkoutParams(t, svc, store, orderID, userID)

	data := map[string]any{
		"transaction": map[string]any{
			"id":        "1234-1610641025-49202",
			"reference": params.Reference,
			"status":    "DECLINED",
		},
	}
	raw := makeWebhookEvent(t, "prod_events_test", []string{"transaction.id", "transaction.status"}, data, 1530291411)

	if err := svc.HandleWebhook(raw); err != nil {
		t.Fatalf("handle webhook: %v", err)
	}

	o, err := store.GetByID(orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if o.PaymentStatus != domain.PaymentStatusFailed {
		t.Errorf("expected failed, got %s", o.PaymentStatus)
	}
	if o.PaidAt != nil {
		t.Error("paid_at must be cleared on failure")
	}
}

func TestHandleWebhook_duplicateApprovalIsIdempotent(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	params := checkoutParams(t, svc, store, orderID, userID)

	data := map[string]any{
		"transaction": map[string]any{
			"id":        "1234-1610641025-49203",
			"reference": params.Reference,
			"status":    "APPROVED",
		},
	}
	raw := makeWebhookEvent(t, "prod_events_test", []string{"transaction.id", "transaction.status"}, data, 1530291411)

	if err := svc.HandleWebhook(raw); err != nil {
		t.Fatalf("first webhook: %v", err)
	}
	first, err := store.GetByID(orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	paidAtFirst := first.PaidAt

	// Same event again (Wompi retries) — must not change anything.
	if err := svc.HandleWebhook(raw); err != nil {
		t.Fatalf("duplicate webhook: %v", err)
	}
	second, err := store.GetByID(orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if second.PaymentStatus != domain.PaymentStatusPaid {
		t.Errorf("expected paid, got %s", second.PaymentStatus)
	}
	if !second.PaidAt.Equal(*paidAtFirst) {
		t.Errorf("duplicate event must not rewrite paid_at: %s vs %s", second.PaidAt, paidAtFirst)
	}
}

func TestHandleWebhook_invalidSignature(t *testing.T) {
	svc, _, _, _ := setupPaymentService(t)
	data := map[string]any{
		"transaction": map[string]any{"id": "tx-1", "reference": "x", "status": "APPROVED"},
	}
	// checksum computed with the WRONG secret
	raw := makeWebhookEvent(t, "wrong-secret", []string{"transaction.id"}, data, 1530291411)

	if err := svc.HandleWebhook(raw); err == nil {
		t.Fatal("expected signature failure")
	}
}

func TestHandleWebhook_unknownReferenceIgnored(t *testing.T) {
	svc, _, _, _ := setupPaymentService(t)
	data := map[string]any{
		"transaction": map[string]any{
			"id":        "tx-1",
			"reference": "no-such-reference",
			"status":    "APPROVED",
		},
	}
	raw := makeWebhookEvent(t, "prod_events_test", []string{"transaction.id", "transaction.reference", "transaction.status"}, data, 1530291411)

	// Not an error: unknown references are ignored (200), logged.
	if err := svc.HandleWebhook(raw); err != nil {
		t.Fatalf("unknown reference must be ignored, got %v", err)
	}
}

func TestHandleWebhook_nonTerminalStatusIgnored(t *testing.T) {
	svc, store, orderID, userID := setupPaymentService(t)
	params := checkoutParams(t, svc, store, orderID, userID)

	data := map[string]any{
		"transaction": map[string]any{
			"id":        "tx-1",
			"reference": params.Reference,
			"status":    "PENDING",
		},
	}
	raw := makeWebhookEvent(t, "prod_events_test", []string{"transaction.id", "transaction.status"}, data, 1530291411)

	if err := svc.HandleWebhook(raw); err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	o, _ := store.GetByID(orderID)
	if o.PaymentStatus != domain.PaymentStatusPending {
		t.Errorf("expected pending, got %s", o.PaymentStatus)
	}
}

// If BaseURL is empty the checkout form gets an empty redirect URL and
// Wompi shows its own success page (the webhook is the source of truth).
func TestCreateCheckout_withoutBaseURL_omitsRedirect(t *testing.T) {
	svc := NewPaymentService(memstore.NewOrderStore(), memstore.NewUserStore(), PaymentConfig{
		PublicKey:       "pub_test_xxx",
		IntegritySecret: "prod_integrity_test",
		EventSecret:     "prod_events_test",
		BaseURL:         "",
	})
	_ = svc // construction smoke check
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()
	user, err := userStore.Create(domain.User{Email: "no-base@example.com", Name: "No Base"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	order := domain.Order{
		UserID:          user.ID,
		Status:          domain.OrderStatusConfirmed,
		TotalCents:      1000,
		Currency:        "COP",
		ShippingStreet:  "Calle 1",
		ShippingCity:    "Bogotá",
		ShippingDept:    "Cundinamarca",
		ShippingCountry: "CO",
		Items: []domain.OrderItem{
			{CoffeeID: availCoffeeID, CoffeeName: "El Paraíso 92", Quantity: 1, UnitPriceCents: 1000},
		},
	}
	created, err := orderStore.Create(order)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	svc = NewPaymentService(orderStore, userStore, PaymentConfig{
		PublicKey:       "pub_test_xxx",
		IntegritySecret: "prod_integrity_test",
		EventSecret:     "prod_events_test",
	})
	params, err := svc.CreateCheckout(created.ID, user.ID)
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if params.RedirectURL != "" {
		t.Errorf("redirect URL must be empty when no BaseURL is set, got %q", params.RedirectURL)
	}
}
