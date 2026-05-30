package handler

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"encoding/json"
	"errors"
	"net/http"
)

type CartHandler struct {
	service *service.CartService
}

func NewCartHandler(s *service.CartService) *CartHandler {
	return &CartHandler{service: s}
}

type addItemRequest struct {
	CoffeeID string `json:"coffee_id"`
	Quantity int    `json:"quantity"`
}

type setQuantityRequest struct {
	Quantity int `json:"quantity"`
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "X-Session-ID header required for guest cart", "BAD_REQUEST")
		return
	}

	cart, err := h.service.GetCart(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "X-Session-ID header required for guest cart", "BAD_REQUEST")
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}
	if req.CoffeeID == "" || req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "coffee_id and quantity > 0 are required", "BAD_REQUEST")
		return
	}

	cart, err := h.service.AddItem(id, req.CoffeeID, req.Quantity)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusUnprocessableEntity, "coffee is not available", "UNAVAILABLE")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (h *CartHandler) SetQuantity(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "X-Session-ID header required for guest cart", "BAD_REQUEST")
		return
	}

	coffeeID := r.PathValue("coffeeId")
	if coffeeID == "" {
		writeError(w, http.StatusBadRequest, "missing coffeeId", "BAD_REQUEST")
		return
	}

	var req setQuantityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}
	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "quantity must be > 0", "BAD_REQUEST")
		return
	}

	cart, err := h.service.SetQuantity(id, coffeeID, req.Quantity)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "item not found in cart", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "X-Session-ID header required for guest cart", "BAD_REQUEST")
		return
	}

	coffeeID := r.PathValue("coffeeId")
	if coffeeID == "" {
		writeError(w, http.StatusBadRequest, "missing coffeeId", "BAD_REQUEST")
		return
	}

	cart, err := h.service.RemoveItem(id, coffeeID)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "item not found in cart", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "X-Session-ID header required for guest cart", "BAD_REQUEST")
		return
	}

	if err := h.service.Clear(id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveIdentity devuelve CartIdentity priorizando usuario autenticado (JWT en context)
// sobre invitado (X-Session-ID header). Devuelve false si no hay ninguna identidad.
func (h *CartHandler) resolveIdentity(r *http.Request) (domain.CartIdentity, bool) {
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		return domain.CartIdentity{ID: userID, IsGuest: false}, true
	}
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		return domain.CartIdentity{}, false
	}
	return domain.CartIdentity{ID: sessionID, IsGuest: true}, true
}
