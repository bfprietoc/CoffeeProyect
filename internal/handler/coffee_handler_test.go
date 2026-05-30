package handler

import (
	"coffeeproyect/internal/service"
	"coffeeproyect/internal/store/memory"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestCoffeeHandler() *CoffeeHandler {
	return NewCoffeeHandler(service.NewCoffeeService(memory.NewCoffeeStore()))
}

func TestCoffeeHandler_GetByID_ok(t *testing.T) {
	h := newTestCoffeeHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /coffees/{id}", h.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/coffees/c1a2b3c4-0001-0001-0001-000000000001", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["name"] != "El Paraíso 92" {
		t.Errorf("unexpected name: %v", body["name"])
	}
}

func TestCoffeeHandler_GetByID_notFound(t *testing.T) {
	h := newTestCoffeeHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /coffees/{id}", h.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/coffees/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["code"] != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %v", body["code"])
	}
}

func TestCoffeeHandler_List_ok(t *testing.T) {
	h := newTestCoffeeHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /coffees", h.List)

	req := httptest.NewRequest(http.MethodGet, "/coffees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body []any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 5 {
		t.Errorf("expected 5 coffees, got %d", len(body))
	}
}

func TestCoffeeHandler_List_filterAvailable(t *testing.T) {
	h := newTestCoffeeHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /coffees", h.List)

	req := httptest.NewRequest(http.MethodGet, "/coffees?available=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body []any
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 4 {
		t.Errorf("expected 4 available coffees, got %d", len(body))
	}
}

func TestCoffeeHandler_List_invalidAvailable(t *testing.T) {
	h := newTestCoffeeHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /coffees", h.List)

	req := httptest.NewRequest(http.MethodGet, "/coffees?available=notabool", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCoffeeHandler_List_invalidLimit(t *testing.T) {
	h := newTestCoffeeHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /coffees", h.List)

	req := httptest.NewRequest(http.MethodGet, "/coffees?limit=999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
