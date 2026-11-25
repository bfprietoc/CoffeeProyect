package handler

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	memstore "coffeeproyect/internal/store/memory"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newPaymentTestEnv(t *testing.T, enabled bool) (*http.ServeMux, string, string) {
	t.Helper()
	userStore := memstore.NewUserStore()
	orderStore := memstore.NewOrderStore()
	userSvc := service.NewUserService(userStore, testSecret)
	orderH := NewOrderHandler(service.NewOrderService(orderStore))
	authH := NewAuthHandler(userSvc, nil)
	requireAuth := middleware.RequireAuth(testSecret)

	var cfg service.PaymentConfig
	if enabled {
		cfg = service.PaymentConfig{
			PublicKey:       "pub_test_xxx",
			IntegritySecret: "prod_integrity_test",
			EventSecret:     "prod_events_test",
			BaseURL:         "http://localhost:3001",
		}
	}
	paymentSvc := service.NewPaymentService(orderStore, userStore, cfg)
	paymentH := NewPaymentHandler(paymentSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", authH.Register)
	mux.Handle("GET /orders", requireAuth(http.HandlerFunc(orderH.List)))
	mux.Handle("POST /orders/{id}/payment", requireAuth(http.HandlerFunc(paymentH.CreatePayment)))
	mux.Handle("POST /webhooks/wompi", http.HandlerFunc(paymentH.Webhook))

	// register the user and capture token + user id
	rec := post(mux, "/auth/register", map[string]string{
		"name": "Payment User", "email": "payment@test.com", "password": "password123",
	}, "")
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	token := body["tokens"].(map[string]any)["access_token"].(string)
	userID := body["user"].(map[string]any)["id"].(string)

	created, _ := orderStore.Create(domain.Order{
		UserID: userID, Status: domain.OrderStatusConfirmed,
		TotalCents: 8500000, Currency: "COP",
		ShippingStreet: "Calle 1", ShippingCity: "Bogotá",
		ShippingDept: "Cundi", ShippingCountry: "CO",
		Items: []domain.OrderItem{
			{CoffeeID: testCoffeeID, CoffeeName: "El Paraíso", Quantity: 1, UnitPriceCents: 8500000},
		},
	})
	return mux, token, created.ID
}

func TestPaymentHandler_CreatePayment_disabled(t *testing.T) {
	mux, token, orderID := newPaymentTestEnv(t, false)

	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/payment", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d — %s", rec.Code, rec.Body)
	}
	var errBody map[string]any
	json.NewDecoder(rec.Body).Decode(&errBody)
	if errBody["code"] != "PAYMENTS_DISABLED" {
		t.Errorf("expected PAYMENTS_DISABLED, got %v", errBody["code"])
	}
}

func TestPaymentHandler_CreatePayment_success(t *testing.T) {
	mux, token, orderID := newPaymentTestEnv(t, true)

	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/payment", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["publicKey"] != "pub_test_xxx" {
		t.Errorf("wrong publicKey: %v", body["publicKey"])
	}
	if body["amountInCents"].(float64) != 8500000 {
		t.Errorf("wrong amountInCents: %v", body["amountInCents"])
	}
	if body["currency"] != "COP" {
		t.Errorf("wrong currency: %v", body["currency"])
	}
	if body["signature"] == "" {
		t.Error("signature must not be empty")
	}
	if body["reference"] == "" {
		t.Error("reference must not be empty")
	}
	if body["redirectUrl"] == "" {
		t.Error("redirectUrl must not be empty")
	}
	if body["customerEmail"] != "payment@test.com" {
		t.Errorf("customerEmail should come from the profile: %v", body["customerEmail"])
	}
}

func TestPaymentHandler_CreatePayment_requiresAuth(t *testing.T) {
	mux, _, orderID := newPaymentTestEnv(t, true)

	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/payment", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestPaymentHandler_CreatePayment_notFound(t *testing.T) {
	mux, token, _ := newPaymentTestEnv(t, true)

	req := httptest.NewRequest(http.MethodPost, "/orders/00000000-0000-0000-0000-000000000000/payment", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d — %s", rec.Code, rec.Body)
	}
}

func TestPaymentHandler_Webhook_invalidSignature(t *testing.T) {
	mux, _, _ := newPaymentTestEnv(t, true)

	payload := map[string]any{
		"event":       "transaction.updated",
		"environment": "test",
		"timestamp":   1530291411,
		"data":        map[string]any{"transaction": map[string]any{"id": "tx-1", "reference": "ref-1", "status": "APPROVED"}},
		"signature":   map[string]any{"properties": []string{"transaction.id"}, "checksum": "deadbeef"},
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/wompi", strings.NewReader(string(raw)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad checksum, got %d — %s", rec.Code, rec.Body)
	}
}

func TestPaymentHandler_Webhook_validButUnknownReference(t *testing.T) {
	mux, _, _ := newPaymentTestEnv(t, true)

	// build a valid checksum for a reference we never created
	secret := "prod_events_test"
	chain := "tx-1APPROVED" + "1530291411" + secret
	sum := sha256.Sum256([]byte(chain))
	payload := map[string]any{
		"event":       "transaction.updated",
		"environment": "test",
		"timestamp":   1530291411,
		"data":        map[string]any{"transaction": map[string]any{"id": "tx-1", "reference": "unknown-ref", "status": "APPROVED"}},
		"signature":   map[string]any{"properties": []string{"transaction.id", "transaction.status"}, "checksum": hex.EncodeToString(sum[:])},
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/wompi", strings.NewReader(string(raw)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("unknown reference must still yield 200 (Wompi retries on non-200), got %d — %s", rec.Code, rec.Body)
	}
}

var _ = fmt.Sprintf // keep fmt import for future helper use
