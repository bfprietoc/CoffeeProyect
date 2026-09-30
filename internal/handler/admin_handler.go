package handler

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/service"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type AdminHandler struct {
	stockSvc  *service.StockService
	orderSvc  *service.OrderService
	coffeeSvc *service.CoffeeService
}

func NewAdminHandler(stock *service.StockService, orders *service.OrderService) *AdminHandler {
	return &AdminHandler{stockSvc: stock, orderSvc: orders}
}

func (h *AdminHandler) WithCoffeeService(coffee *service.CoffeeService) *AdminHandler {
	h.coffeeSvc = coffee
	return h
}

// POST /admin/coffees
func (h *AdminHandler) CreateCoffee(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		Process      string   `json:"process"`
		RoastLevel   string   `json:"roast_level"`
		Description  string   `json:"description"`
		TastingNotes []string `json:"tasting_notes"`
		BagSizeGrams int      `json:"bag_size_grams"`
		PriceCents   int      `json:"price_cents"`
		Currency     string   `json:"currency"`
		StockBags    int      `json:"stock_bags"`
		ProducerName string   `json:"producer_name"`
		FarmName     string   `json:"farm_name"`
		FarmCountry  string   `json:"farm_country"`
		FarmRegion   string   `json:"farm_region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body", "BAD_REQUEST")
		return
	}

	input := domain.Coffee{
		Name:         req.Name,
		Process:      req.Process,
		RoastLevel:   req.RoastLevel,
		Description:  req.Description,
		TastingNotes: req.TastingNotes,
		BagSizeGrams: req.BagSizeGrams,
		PriceCents:   req.PriceCents,
		Currency:     req.Currency,
		StockBags:    req.StockBags,
		Producer:     domain.Producer{Name: req.ProducerName},
		Farm:         domain.Farm{Name: req.FarmName, Country: req.FarmCountry, Region: req.FarmRegion},
	}

	created, err := h.coffeeSvc.Create(input)
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "name, process, roast_level and price_cents are required", "BAD_REQUEST")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// PUT /admin/coffees/{id}
func (h *AdminHandler) UpdateCoffee(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name         string   `json:"name"`
		Process      string   `json:"process"`
		RoastLevel   string   `json:"roast_level"`
		Description  string   `json:"description"`
		TastingNotes []string `json:"tasting_notes"`
		BagSizeGrams int      `json:"bag_size_grams"`
		PriceCents   int      `json:"price_cents"`
		Currency     string   `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body", "BAD_REQUEST")
		return
	}

	input := domain.Coffee{
		Name:         req.Name,
		Process:      req.Process,
		RoastLevel:   req.RoastLevel,
		Description:  req.Description,
		TastingNotes: req.TastingNotes,
		BagSizeGrams: req.BagSizeGrams,
		PriceCents:   req.PriceCents,
		Currency:     req.Currency,
	}

	updated, err := h.coffeeSvc.Update(id, input)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "name, process, roast_level and price_cents are required", "BAD_REQUEST")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DELETE /admin/coffees/{id}
func (h *AdminHandler) DeleteCoffee(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := h.coffeeSvc.Delete(id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /admin/coffees/{id}/stock
func (h *AdminHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if h.coffeeSvc != nil {
		if _, err := h.coffeeSvc.GetByID(id); errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
			return
		}
	}

	n, err := h.stockSvc.GetStock(id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	movements, err := h.stockSvc.GetMovements(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"coffee_id":  id,
		"stock_bags": n,
		"movements":  movements,
	})
}

// PATCH /admin/coffees/{id}/stock
func (h *AdminHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Operation string `json:"operation"` // "add" or "subtract"
		Quantity  int    `json:"quantity"`
		Note      string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body", "BAD_REQUEST")
		return
	}
	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "quantity must be positive", "BAD_REQUEST")
		return
	}
	if req.Operation != "add" && req.Operation != "subtract" {
		writeError(w, http.StatusBadRequest, "operation must be 'add' or 'subtract'", "BAD_REQUEST")
		return
	}

	delta := req.Quantity
	if req.Operation == "subtract" {
		delta = -req.Quantity
	}

	result, err := h.stockSvc.Adjust(id, delta, req.Note)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "coffee not found", "NOT_FOUND")
		return
	}
	if errors.Is(err, domain.ErrInsufficientStock) {
		writeError(w, http.StatusUnprocessableEntity, "not enough stock to subtract", "UNAVAILABLE")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"coffee_id":    id,
		"old_stock":    result.OldStock,
		"result_stock": result.ResultStock,
	})
}

// GET /admin/orders
func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	orders, err := h.orderSvc.ListAll(statusFilter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

// PATCH /admin/orders/{id}/status
func (h *AdminHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Status         string  `json:"status"`
		TrackingNumber *string `json:"tracking_number,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required", "BAD_REQUEST")
		return
	}

	order, err := h.orderSvc.UpdateStatus(id, domain.OrderStatus(req.Status), req.TrackingNumber)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, order)
}
