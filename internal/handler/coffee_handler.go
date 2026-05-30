package handler

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"coffeeproyect/internal/store"
	"errors"
	"net/http"
	"strconv"
)

type CoffeeHandler struct {
	service *service.CoffeeService
}

func NewCoffeeHandler(s *service.CoffeeService) *CoffeeHandler {
	return &CoffeeHandler{service: s}
}

func (h *CoffeeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id", "BAD_REQUEST")
		return
	}

	coffee, err := h.service.GetByID(id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, coffee)
}

func (h *CoffeeHandler) List(w http.ResponseWriter, r *http.Request) {
	filters := store.CoffeeFilters{
		RoastLevel: r.URL.Query().Get("roast_level"),
		Country:    r.URL.Query().Get("country"),
		Process:    r.URL.Query().Get("process"),
	}

	if v := r.URL.Query().Get("available"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "available must be true or false", "BAD_REQUEST")
			return
		}
		filters.Available = &b
	}

	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "page must be a positive integer", "BAD_REQUEST")
			return
		}
		filters.Page = n
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 50", "BAD_REQUEST")
			return
		}
		filters.Limit = n
	}

	coffees, err := h.service.List(filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, coffees)
}
