package handler

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"encoding/json"
	"errors"
	"net/http"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(s *service.UserService) *UserHandler {
	return &UserHandler{service: s}
}

type updateProfileRequest struct {
	Name string `json:"name"`
}

type addAddressRequest struct {
	Label      string `json:"label"`
	Street     string `json:"street"`
	City       string `json:"city"`
	Department string `json:"department"`
	Country    string `json:"country"`
	IsDefault  bool   `json:"is_default"`
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	user, addresses, err := h.service.GetProfile(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"addresses":  addresses,
		"created_at": user.CreatedAt,
	})
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	user, err := h.service.UpdateProfile(userID, req.Name)
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "name is required", "BAD_REQUEST")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) AddAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req addAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}
	if req.Label == "" || req.Street == "" || req.City == "" || req.Department == "" {
		writeError(w, http.StatusBadRequest, "label, street, city and department are required", "BAD_REQUEST")
		return
	}

	country := req.Country
	if country == "" {
		country = "CO"
	}

	address, err := h.service.AddAddress(userID, domain.Address{
		Label:      req.Label,
		Street:     req.Street,
		City:       req.City,
		Department: req.Department,
		Country:    country,
		IsDefault:  req.IsDefault,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, address)
}

func (h *UserHandler) GetAddresses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	addresses, err := h.service.GetAddresses(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, addresses)
}

func (h *UserHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	addressID := r.PathValue("id")
	if addressID == "" {
		writeError(w, http.StatusBadRequest, "missing address id", "BAD_REQUEST")
		return
	}

	if err := h.service.DeleteAddress(addressID, userID); errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "address not found", "NOT_FOUND")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
