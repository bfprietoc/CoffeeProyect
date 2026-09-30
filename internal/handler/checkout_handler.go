package handler

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"encoding/json"
	"errors"
	"net/http"
)

type CheckoutHandler struct {
	svc *service.CheckoutService
}

func NewCheckoutHandler(svc *service.CheckoutService) *CheckoutHandler {
	return &CheckoutHandler{svc: svc}
}

func (h *CheckoutHandler) Validate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	result, err := h.svc.Validate(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CheckoutHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req struct {
		AddressID string `json:"address_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AddressID == "" {
		writeError(w, http.StatusBadRequest, "address_id is required", "BAD_REQUEST")
		return
	}

	order, err := h.svc.PlaceOrder(userID, req.AddressID)
	if errors.Is(err, domain.ErrCartEmpty) {
		writeError(w, http.StatusUnprocessableEntity, "cart is empty", "BAD_REQUEST")
		return
	}
	if errors.Is(err, domain.ErrInsufficientStock) {
		writeError(w, http.StatusUnprocessableEntity, "one or more items are out of stock", "UNAVAILABLE")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "address not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, order)
}
