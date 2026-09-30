package handler

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	memstore "coffeeproyect/internal/store/memory"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newOrderTestEnv returns an authenticated mux with a pre-seeded order.
func newOrderTestEnv(t *testing.T) (mux *http.ServeMux, token string, orderID string) {
	t.Helper()
	userStore := memstore.NewUserStore()
	orderStore := memstore.NewOrderStore()
	userSvc := service.NewUserService(userStore, testSecret)
	orderSvc := service.NewOrderService(orderStore)
	orderH := NewOrderHandler(orderSvc)
	authH := NewAuthHandler(userSvc, nil)
	requireAuth := middleware.RequireAuth(testSecret)

	mux = http.NewServeMux()
	mux.HandleFunc("POST /auth/register", authH.Register)
	mux.Handle("GET /orders", requireAuth(http.HandlerFunc(orderH.List)))
	mux.Handle("GET /orders/{id}", requireAuth(http.HandlerFunc(orderH.GetByID)))
	mux.Handle("POST /orders/{id}/cancel", requireAuth(http.HandlerFunc(orderH.Cancel)))

	// register user and capture token
	rec := post(mux, "/auth/register", map[string]string{
		"name": "Order User", "email": "orders@test.com", "password": "password123",
	}, "")
	var regBody map[string]any
	json.NewDecoder(rec.Body).Decode(&regBody)
	token = regBody["tokens"].(map[string]any)["access_token"].(string)
	userID := regBody["user"].(map[string]any)["id"].(string)

	// pre-seed an order for this user
	created, _ := orderStore.Create(domain.Order{
		UserID: userID, Status: domain.OrderStatusConfirmed,
		TotalCents: 8500000, Currency: "COP",
		ShippingStreet: "Calle 1", ShippingCity: "Bogotá",
		ShippingDept: "Cundi", ShippingCountry: "CO",
		Items: []domain.OrderItem{
			{CoffeeID: testCoffeeID, CoffeeName: "El Paraíso", Quantity: 1, UnitPriceCents: 8500000},
		},
	})
	orderID = created.ID
	return
}

func TestOrderHandler_List(t *testing.T) {
	mux, token, _ := newOrderTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var orders []any
	json.NewDecoder(rec.Body).Decode(&orders)
	if len(orders) != 1 {
		t.Errorf("expected 1 order, got %d", len(orders))
	}
}

func TestOrderHandler_GetByID_found(t *testing.T) {
	mux, token, orderID := newOrderTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
}

func TestOrderHandler_GetByID_notFound(t *testing.T) {
	mux, token, _ := newOrderTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/orders/nonexistent-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestOrderHandler_Cancel_success(t *testing.T) {
	mux, token, orderID := newOrderTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var order map[string]any
	json.NewDecoder(rec.Body).Decode(&order)
	if order["status"] != "cancelled" {
		t.Errorf("expected cancelled status, got %v", order["status"])
	}
}

func TestOrderHandler_Cancel_noToken(t *testing.T) {
	mux, _, orderID := newOrderTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/cancel", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
