package handler

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"encoding/json"
	"errors"
	"net/http"
)

type AuthHandler struct {
	userService *service.UserService
	cartService *service.CartService // opcional: fusiona carrito invitado al login
}

func NewAuthHandler(users *service.UserService, cart *service.CartService) *AuthHandler {
	return &AuthHandler{userService: users, cartService: cart}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	tokens, user, err := h.userService.Register(req.Name, req.Email, req.Password)
	if errors.Is(err, domain.ErrEmailAlreadyExists) {
		writeError(w, http.StatusConflict, "email already registered", "EMAIL_CONFLICT")
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "name, email and password (min 8 chars) are required", "BAD_REQUEST")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	tokens, user, err := h.userService.Login(req.Email, req.Password)
	if errors.Is(err, domain.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "invalid credentials", "UNAUTHORIZED")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	// fusionar carrito de invitado si viene X-Session-ID
	if h.cartService != nil {
		if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
			_ = h.cartService.MergeGuestCart(sessionID, user.ID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	tokens, err := h.userService.RefreshToken(req.RefreshToken)
	if errors.Is(err, domain.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token", "UNAUTHORIZED")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}
