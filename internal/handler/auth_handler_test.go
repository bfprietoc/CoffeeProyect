package handler

import (
	"bytes"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	"coffeeproyect/internal/store/memory"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSecret = "test-secret"

func newTestUserService() *service.UserService {
	return service.NewUserService(memory.NewUserStore(), testSecret)
}

func newTestMux() *http.ServeMux {
	svc := newTestUserService()
	authH := NewAuthHandler(svc, nil)
	userH := NewUserHandler(svc)
	requireAuth := middleware.RequireAuth(testSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", authH.Register)
	mux.HandleFunc("POST /auth/login", authH.Login)
	mux.HandleFunc("POST /auth/refresh", authH.Refresh)
	mux.Handle("GET /users/me", requireAuth(http.HandlerFunc(userH.GetProfile)))
	mux.Handle("POST /users/me/addresses", requireAuth(http.HandlerFunc(userH.AddAddress)))
	mux.Handle("GET /users/me/addresses", requireAuth(http.HandlerFunc(userH.GetAddresses)))
	mux.Handle("DELETE /users/me/addresses/{id}", requireAuth(http.HandlerFunc(userH.DeleteAddress)))
	return mux
}

func post(mux *http.ServeMux, path string, body any, token string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func get(mux *http.ServeMux, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_success(t *testing.T) {
	rec := post(newTestMux(), "/auth/register", map[string]string{
		"name": "Brayan", "email": "b@test.com", "password": "password123",
	}, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — %s", rec.Code, rec.Body)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["tokens"] == nil {
		t.Error("expected tokens in response")
	}
	if body["user"] == nil {
		t.Error("expected user in response")
	}
}

func TestRegister_duplicateEmail(t *testing.T) {
	mux := newTestMux()
	payload := map[string]string{"name": "A", "email": "dup@test.com", "password": "password123"}

	post(mux, "/auth/register", payload, "")
	rec := post(mux, "/auth/register", payload, "")

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestRegister_missingFields(t *testing.T) {
	rec := post(newTestMux(), "/auth/register", map[string]string{
		"email": "x@test.com",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_shortPassword(t *testing.T) {
	rec := post(newTestMux(), "/auth/register", map[string]string{
		"name": "A", "email": "x@test.com", "password": "short",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_success(t *testing.T) {
	mux := newTestMux()
	post(mux, "/auth/register", map[string]string{
		"name": "Brayan", "email": "login@test.com", "password": "password123",
	}, "")

	rec := post(mux, "/auth/login", map[string]string{
		"email": "login@test.com", "password": "password123",
	}, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	tokens, _ := body["tokens"].(map[string]any)
	if tokens["access_token"] == "" {
		t.Error("expected access_token in tokens")
	}
}

func TestLogin_wrongPassword(t *testing.T) {
	mux := newTestMux()
	post(mux, "/auth/register", map[string]string{
		"name": "A", "email": "wp@test.com", "password": "password123",
	}, "")

	rec := post(mux, "/auth/login", map[string]string{
		"email": "wp@test.com", "password": "wrongpassword",
	}, "")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogin_unknownEmail(t *testing.T) {
	rec := post(newTestMux(), "/auth/login", map[string]string{
		"email": "nobody@test.com", "password": "password123",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// ── Auth middleware ───────────────────────────────────────────────────────────

func TestGetProfile_noToken(t *testing.T) {
	rec := get(newTestMux(), "/users/me", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetProfile_invalidToken(t *testing.T) {
	rec := get(newTestMux(), "/users/me", "not.a.valid.token")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetProfile_success(t *testing.T) {
	mux := newTestMux()

	regRec := post(mux, "/auth/register", map[string]string{
		"name": "Brayan", "email": "profile@test.com", "password": "password123",
	}, "")

	var regBody map[string]any
	json.NewDecoder(regRec.Body).Decode(&regBody)
	tokens := regBody["tokens"].(map[string]any)
	accessToken := tokens["access_token"].(string)

	rec := get(mux, "/users/me", accessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["email"] != "profile@test.com" {
		t.Errorf("expected email profile@test.com, got %v", body["email"])
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefresh_success(t *testing.T) {
	mux := newTestMux()
	regRec := post(mux, "/auth/register", map[string]string{
		"name": "A", "email": "refresh@test.com", "password": "password123",
	}, "")

	var regBody map[string]any
	json.NewDecoder(regRec.Body).Decode(&regBody)
	tokens := regBody["tokens"].(map[string]any)
	refreshToken := tokens["refresh_token"].(string)

	rec := post(mux, "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	}, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}
}

func TestRefresh_invalidToken(t *testing.T) {
	rec := post(newTestMux(), "/auth/refresh", map[string]string{
		"refresh_token": "invalid.token",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// ── Addresses ─────────────────────────────────────────────────────────────────

func TestAddAddress_success(t *testing.T) {
	mux := newTestMux()
	regRec := post(mux, "/auth/register", map[string]string{
		"name": "A", "email": "addr@test.com", "password": "password123",
	}, "")

	var regBody map[string]any
	json.NewDecoder(regRec.Body).Decode(&regBody)
	token := regBody["tokens"].(map[string]any)["access_token"].(string)

	rec := post(mux, "/users/me/addresses", map[string]any{
		"label": "casa", "street": "Calle 123", "city": "Bogotá",
		"department": "Cundinamarca", "country": "CO", "is_default": true,
	}, token)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — %s", rec.Code, rec.Body)
	}
}

func TestAddAddress_noToken(t *testing.T) {
	rec := post(newTestMux(), "/users/me/addresses", map[string]any{
		"label": "casa", "street": "Calle 1", "city": "Bogotá", "department": "Cundi",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
