package handler

import (
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	memstore "coffeeproyect/internal/store/memory"
	"encoding/json"
	"net/http"
	"testing"
)

func newWaitlistTestMux() *http.ServeMux {
	coffeeStore := memstore.NewCoffeeStore()
	waitlistStore := memstore.NewWaitlistStore()
	svc := service.NewWaitlistService(waitlistStore, coffeeStore, nil, nil)
	h := NewWaitlistHandler(svc)
	optionalAuth := middleware.OptionalAuth(testSecret)

	mux := http.NewServeMux()
	mux.Handle("POST /waitlist", optionalAuth(http.HandlerFunc(h.Subscribe)))
	return mux
}

func TestWaitlistHandler_Subscribe_success(t *testing.T) {
	mux := newWaitlistTestMux()

	rec := post(mux, "/waitlist", map[string]string{
		"coffee_id": noStockCoffeeID,
		"email":     "fan@example.com",
	}, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — %s", rec.Code, rec.Body)
	}
	var entry map[string]any
	json.NewDecoder(rec.Body).Decode(&entry)
	if entry["email"] != "fan@example.com" {
		t.Errorf("expected email in response, got %v", entry)
	}
}

func TestWaitlistHandler_Subscribe_coffeeInStock(t *testing.T) {
	mux := newWaitlistTestMux()

	rec := post(mux, "/waitlist", map[string]string{
		"coffee_id": testCoffeeID,
		"email":     "fan@example.com",
	}, "")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for in-stock coffee, got %d", rec.Code)
	}
}

func TestWaitlistHandler_Subscribe_unknownCoffee(t *testing.T) {
	mux := newWaitlistTestMux()

	rec := post(mux, "/waitlist", map[string]string{
		"coffee_id": "nonexistent-coffee",
		"email":     "fan@example.com",
	}, "")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown coffee, got %d", rec.Code)
	}
}

func TestWaitlistHandler_Subscribe_missingFields(t *testing.T) {
	mux := newWaitlistTestMux()

	rec := post(mux, "/waitlist", map[string]string{
		"coffee_id": noStockCoffeeID,
		// missing email
	}, "")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing email, got %d", rec.Code)
	}
}

func TestWaitlistHandler_Subscribe_withAuthToken(t *testing.T) {
	coffeeStore := memstore.NewCoffeeStore()
	waitlistStore := memstore.NewWaitlistStore()
	userStore := memstore.NewUserStore()
	svc := service.NewWaitlistService(waitlistStore, coffeeStore, nil, nil)
	userSvc := service.NewUserService(userStore, testSecret)
	waitlistH := NewWaitlistHandler(svc)
	authH := NewAuthHandler(userSvc, nil)
	optionalAuth := middleware.OptionalAuth(testSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", authH.Register)
	mux.Handle("POST /waitlist", optionalAuth(http.HandlerFunc(waitlistH.Subscribe)))

	// register to get a token
	rec := post(mux, "/auth/register", map[string]string{
		"name": "Coffee Fan", "email": "fan2@test.com", "password": "pass1234",
	}, "")
	var regBody map[string]any
	json.NewDecoder(rec.Body).Decode(&regBody)
	token := regBody["tokens"].(map[string]any)["access_token"].(string)

	// subscribe with JWT using post helper
	w := post(mux, "/waitlist", map[string]string{
		"coffee_id": noStockCoffeeID,
		"email":     "fan2@test.com",
	}, token)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 with auth, got %d — %s", w.Code, w.Body)
	}
	var entry map[string]any
	json.NewDecoder(w.Body).Decode(&entry)
	if entry["user_id"] == nil {
		t.Error("expected user_id to be set when authenticated")
	}
}
