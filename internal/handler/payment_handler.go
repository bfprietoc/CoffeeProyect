package handler

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"errors"
	"io"
	"net/http"
)

// PaymentHandler exposes the Wompi Web Checkout flow.
type PaymentHandler struct {
	svc *service.PaymentService
}

func NewPaymentHandler(svc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// CreatePayment builds the Web Checkout params for the order the user invites
// to pay. The frontend renders Wompi's form with exactly this payload — the
// integrity secret never leaves the server.
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	id := r.PathValue("id")
	params, err := h.svc.CreateCheckout(id, userID)
	if errors.Is(err, domain.ErrPaymentDisabled) {
		writeError(w, http.StatusServiceUnavailable, "payments are not available", "PAYMENTS_DISABLED")
		return
	}
	if errors.Is(err, domain.ErrPaymentAlreadyPaid) {
		writeError(w, http.StatusConflict, "order is already paid", "PAYMENT_ALREADY_PAID")
		return
	}
	if errors.Is(err, domain.ErrPaymentUnavailable) {
		writeError(w, http.StatusUnprocessableEntity, "order cannot be paid in its current state", "PAYMENT_UNAVAILABLE")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, params)
}

// Webhook is public (no JWT): Wompi calls it when a transaction reaches a
// final state. The event checksum is validated inside the service.
func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body", "BAD_REQUEST")
		return
	}
	if err := h.svc.HandleWebhook(body); err != nil {
		// Signature failures → 401 (do not acknowledge). Config errors → 503.
		if errors.Is(err, domain.ErrPaymentDisabled) {
			writeError(w, http.StatusServiceUnavailable, "payments are not available", "PAYMENTS_DISABLED")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid signature", "BAD_SIGNATURE")
		return
	}
	// Always 200 (even for ignored events): a non-200 makes Wompi retry.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
