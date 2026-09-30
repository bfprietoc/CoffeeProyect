package handler

import (
	"bytes"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	memstore "coffeeproyect/internal/store/memory"
	memorycache "coffeeproyect/internal/cache/memory"
	"coffeeproyect/internal/event"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testAdminKey = "admin-secret"

func newAdminMux() (*http.ServeMux, *memstore.OrderStore) {
	coffeeStore := memstore.NewCoffeeStore()
	stockStore := memstore.NewStockStore()
	orderStore := memstore.NewOrderStore()
	bus := event.NewBus()
	coffeeSvc := service.NewCoffeeService(coffeeStore).WithCache(memorycache.NewCoffeeCache())
	stockSvc := service.NewStockService(stockStore, coffeeSvc, bus)
	orderSvc := service.NewOrderService(orderStore)
	adminH := NewAdminHandler(stockSvc, orderSvc).WithCoffeeService(coffeeSvc)
	requireAdmin := middleware.RequireAdmin(testAdminKey)

	mux := http.NewServeMux()
	mux.Handle("POST /admin/coffees", requireAdmin(http.HandlerFunc(adminH.CreateCoffee)))
	mux.Handle("PUT /admin/coffees/{id}", requireAdmin(http.HandlerFunc(adminH.UpdateCoffee)))
	mux.Handle("DELETE /admin/coffees/{id}", requireAdmin(http.HandlerFunc(adminH.DeleteCoffee)))
	mux.Handle("GET /admin/coffees/{id}/stock", requireAdmin(http.HandlerFunc(adminH.GetStock)))
	mux.Handle("PATCH /admin/coffees/{id}/stock", requireAdmin(http.HandlerFunc(adminH.AdjustStock)))
	mux.Handle("GET /admin/orders", requireAdmin(http.HandlerFunc(adminH.ListOrders)))
	mux.Handle("PATCH /admin/orders/{id}/status", requireAdmin(http.HandlerFunc(adminH.UpdateOrderStatus)))
	return mux, orderStore
}

func adminReq(mux *http.ServeMux, method, path string, body any, apiKey string) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAdmin_GetStock(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodGet, "/admin/coffees/"+testCoffeeID+"/stock", nil, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
}

func TestAdmin_AdjustStock_add(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodPatch, "/admin/coffees/"+testCoffeeID+"/stock",
		map[string]any{"operation": "add", "quantity": 10, "note": "reposición"}, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["result_stock"].(float64) != 10 {
		t.Errorf("expected result_stock=10, got %v", body["result_stock"])
	}
}

func TestAdmin_AdjustStock_subtractBelowZero(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodPatch, "/admin/coffees/"+testCoffeeID+"/stock",
		map[string]any{"operation": "subtract", "quantity": 999, "note": "error"}, testAdminKey)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestAdmin_NoApiKey(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodGet, "/admin/coffees/"+testCoffeeID+"/stock", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAdmin_WrongApiKey(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodGet, "/admin/coffees/"+testCoffeeID+"/stock", nil, "wrong-key")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAdmin_ListOrders(t *testing.T) {
	mux, orderStore := newAdminMux()
	orderStore.Create(domain.Order{
		UserID: "u1", Status: domain.OrderStatusConfirmed, TotalCents: 1000, Currency: "COP",
		ShippingStreet: "St", ShippingCity: "City", ShippingDept: "Dept", ShippingCountry: "CO",
	})

	rec := adminReq(mux, http.MethodGet, "/admin/orders", nil, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var orders []any
	json.NewDecoder(rec.Body).Decode(&orders)
	if len(orders) != 1 {
		t.Errorf("expected 1 order, got %d", len(orders))
	}
}

func TestAdmin_UpdateOrderStatus(t *testing.T) {
	mux, orderStore := newAdminMux()
	created, _ := orderStore.Create(domain.Order{
		UserID: "u1", Status: domain.OrderStatusConfirmed, TotalCents: 1000, Currency: "COP",
		ShippingStreet: "St", ShippingCity: "City", ShippingDept: "Dept", ShippingCountry: "CO",
	})

	tracking := "TRK123"
	rec := adminReq(mux, http.MethodPatch, "/admin/orders/"+created.ID+"/status",
		map[string]any{"status": "shipped", "tracking_number": tracking}, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var order map[string]any
	json.NewDecoder(rec.Body).Decode(&order)
	if order["status"] != "shipped" {
		t.Errorf("expected status shipped, got %v", order["status"])
	}
}

func TestAdmin_CreateCoffee_success(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodPost, "/admin/coffees", map[string]any{
		"name":          "Nuevo Café Test",
		"process":       "washed",
		"roast_level":   "medium",
		"price_cents":   6000000,
		"bag_size_grams": 250,
		"producer_name": "Test Producer",
		"farm_name":     "Test Farm",
		"farm_country":  "Colombia",
		"farm_region":   "Tolima",
	}, testAdminKey)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — %s", rec.Code, rec.Body)
	}
	var coffee map[string]any
	json.NewDecoder(rec.Body).Decode(&coffee)
	if coffee["name"] != "Nuevo Café Test" {
		t.Errorf("expected name 'Nuevo Café Test', got %v", coffee["name"])
	}
	if coffee["id"] == "" {
		t.Error("expected non-empty id")
	}
}

func TestAdmin_CreateCoffee_missingRequiredFields(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodPost, "/admin/coffees",
		map[string]any{"process": "washed"}, testAdminKey)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAdmin_UpdateCoffee_success(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodPut, "/admin/coffees/"+testCoffeeID, map[string]any{
		"name":          "El Paraíso 92 Actualizado",
		"process":       "anaerobic",
		"roast_level":   "light",
		"price_cents":   9000000,
		"bag_size_grams": 250,
	}, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var coffee map[string]any
	json.NewDecoder(rec.Body).Decode(&coffee)
	if coffee["name"] != "El Paraíso 92 Actualizado" {
		t.Errorf("expected updated name, got %v", coffee["name"])
	}
}

func TestAdmin_UpdateCoffee_notFound(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodPut, "/admin/coffees/non-existent", map[string]any{
		"name": "X", "process": "washed", "roast_level": "light", "price_cents": 1000,
	}, testAdminKey)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAdmin_DeleteCoffee_success(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodDelete, "/admin/coffees/"+testCoffeeID, nil, testAdminKey)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d — %s", rec.Code, rec.Body)
	}
	// Verify it's gone
	rec2 := adminReq(mux, http.MethodGet, "/admin/coffees/"+testCoffeeID+"/stock", nil, testAdminKey)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec2.Code)
	}
}

func TestAdmin_DeleteCoffee_notFound(t *testing.T) {
	mux, _ := newAdminMux()
	rec := adminReq(mux, http.MethodDelete, "/admin/coffees/non-existent", nil, testAdminKey)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
