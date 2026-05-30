package handler

import (
	"bytes"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	memstore "coffeeproyect/internal/store/memory"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newCheckoutMux() (*http.ServeMux, *memstore.CartStore, *memstore.UserStore, string) {
	coffeeStore := memstore.NewCoffeeStore()
	cartStore := memstore.NewCartStore()
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()

	userSvc := service.NewUserService(userStore, testSecret)
	checkoutSvc := service.NewCheckoutService(cartStore, coffeeStore, orderStore, userStore)
	orderSvc := service.NewOrderService(orderStore)

	checkoutH := NewCheckoutHandler(checkoutSvc)
	orderH := NewOrderHandler(orderSvc)
	requireAuth := middleware.RequireAuth(testSecret)

	mux := http.NewServeMux()
	mux.Handle("GET /checkout/validate", requireAuth(http.HandlerFunc(checkoutH.Validate)))
	mux.Handle("POST /orders", requireAuth(http.HandlerFunc(checkoutH.PlaceOrder)))
	mux.Handle("GET /orders", requireAuth(http.HandlerFunc(orderH.List)))
	mux.Handle("GET /orders/{id}", requireAuth(http.HandlerFunc(orderH.GetByID)))
	mux.Handle("POST /orders/{id}/cancel", requireAuth(http.HandlerFunc(orderH.Cancel)))
	// auth for token generation
	authH := NewAuthHandler(userSvc, nil)
	mux.HandleFunc("POST /auth/register", authH.Register)
	mux.HandleFunc("POST /auth/login", authH.Login)

	return mux, cartStore, userStore, testSecret
}

func registerAndGetToken(t *testing.T, mux *http.ServeMux) (string, string) {
	t.Helper()
	rec := post(mux, "/auth/register", map[string]string{
		"name": "Checkout User", "email": "checkout@test.com", "password": "password123",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d — %s", rec.Code, rec.Body)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	tokens := body["tokens"].(map[string]any)
	user := body["user"].(map[string]any)
	return tokens["access_token"].(string), user["id"].(string)
}

func addAddressAndGetID(t *testing.T, mux *http.ServeMux, token string) string {
	t.Helper()
	rec := post(mux, "/users/me/addresses", map[string]any{
		"label": "casa", "street": "Calle 1", "city": "Bogotá",
		"department": "Cundi", "country": "CO", "is_default": true,
	}, token)

	// we need the address ID — add a user handler to the mux for this
	// Since we can't easily get the address ID from the register response,
	// we'll use the get addresses endpoint if available, or just use the
	// user store directly through a helper.
	_ = rec
	return ""
}

// TestCheckoutHandler_validate tests the validate endpoint via HTTP.
func TestCheckoutHandler_validate(t *testing.T) {
	mux, cartStore, userStore, _ := newCheckoutMux()
	token, userID := registerAndGetToken(t, mux)
	_ = userStore

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: testCoffeeID, CoffeeName: "El Paraíso 92",
		Quantity: 1, UnitPriceCents: 8500000,
	})

	req := httptest.NewRequest(http.MethodGet, "/checkout/validate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["can_checkout"] != true {
		t.Errorf("expected can_checkout=true, got %v", result["can_checkout"])
	}
}

func TestCheckoutHandler_placeOrder_noAddress(t *testing.T) {
	mux, cartStore, _, _ := newCheckoutMux()
	token, userID := registerAndGetToken(t, mux)

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: testCoffeeID, Quantity: 1, UnitPriceCents: 8500000,
	})

	b, _ := json.Marshal(map[string]string{"address_id": "non-existent"})
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing address, got %d", rec.Code)
	}
}

func TestCheckoutHandler_placeOrder_emptyCart(t *testing.T) {
	mux, _, _, _ := newCheckoutMux()
	token, _ := registerAndGetToken(t, mux)

	b, _ := json.Marshal(map[string]string{"address_id": "any"})
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for empty cart, got %d — %s", rec.Code, rec.Body)
	}
}

func TestCheckoutHandler_noToken(t *testing.T) {
	mux, _, _, _ := newCheckoutMux()
	req := httptest.NewRequest(http.MethodGet, "/checkout/validate", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
