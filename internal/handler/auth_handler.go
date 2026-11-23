package handler

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type AuthHandler struct {
	userService *service.UserService
	cartService *service.CartService // opcional: fusiona carrito invitado al login

	googleClientID string // vacío = login con Google deshabilitado (503)
	// googleVerifier valida el ID token de Google. Inyectable para tests;
	// por defecto auth.VerifyGoogleIDToken (verifica contra Google, usa red).
	googleVerifier func(ctx context.Context, idToken, audience string) (auth.GoogleIdentity, error)
}

func NewAuthHandler(users *service.UserService, cart *service.CartService) *AuthHandler {
	return &AuthHandler{
		userService:    users,
		cartService:    cart,
		googleVerifier: auth.VerifyGoogleIDToken,
	}
}

// WithGoogleClientID habilita el login con Google. Sin client ID el
// endpoint /auth/google responde 503 GOOGLE_AUTH_DISABLED.
func (h *AuthHandler) WithGoogleClientID(clientID string) *AuthHandler {
	h.googleClientID = clientID
	return h
}

// WithGoogleVerifier reemplaza el verificador de ID tokens (solo para tests).
func (h *AuthHandler) WithGoogleVerifier(f func(ctx context.Context, idToken, audience string) (auth.GoogleIdentity, error)) *AuthHandler {
	h.googleVerifier = f
	return h
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

type googleLoginRequest struct {
	Credential string `json:"credential"` // ID token (JWT) emitido por Google
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
		writeInternalError(w, err)
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
		writeInternalError(w, err)
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

// GoogleLogin inicia sesión (o crea la cuenta) con un ID token de Google.
// Responde el mismo shape que Login: { user, tokens }.
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if h.googleClientID == "" {
		writeError(w, http.StatusServiceUnavailable, "google login is not configured", "GOOGLE_AUTH_DISABLED")
		return
	}

	var req googleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Credential == "" {
		writeError(w, http.StatusBadRequest, "credential (Google ID token) is required", "BAD_REQUEST")
		return
	}

	identity, err := h.googleVerifier(r.Context(), req.Credential, h.googleClientID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid Google token", "INVALID_GOOGLE_TOKEN")
		return
	}

	tokens, user, err := h.userService.RegisterOrLoginWithGoogle(identity.Email, identity.Name)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	// fusionar carrito de invitado si viene X-Session-ID (igual que Login)
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
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}
