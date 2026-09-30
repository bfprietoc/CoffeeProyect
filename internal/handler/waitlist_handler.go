package handler

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"encoding/json"
	"errors"
	"net/http"
)

type waitlistSubscriber interface {
	Subscribe(coffeeID, email string, userID *string) (domain.WaitlistEntry, error)
}

type WaitlistHandler struct {
	svc waitlistSubscriber
}

func NewWaitlistHandler(svc waitlistSubscriber) *WaitlistHandler {
	return &WaitlistHandler{svc: svc}
}

// POST /waitlist
func (h *WaitlistHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CoffeeID string `json:"coffee_id"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CoffeeID == "" || body.Email == "" {
		writeError(w, http.StatusBadRequest, "coffee_id and email are required", "BAD_REQUEST")
		return
	}

	var userID *string
	if uid, ok := auth.UserIDFromContext(r.Context()); ok {
		userID = &uid
	}

	entry, err := h.svc.Subscribe(body.CoffeeID, body.Email, userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		case errors.Is(err, domain.ErrInvalidInput):
			writeError(w, http.StatusUnprocessableEntity, err.Error(), "INVALID_INPUT")
		default:
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		}
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}
