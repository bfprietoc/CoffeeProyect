package handler

import (
	"bytes"
	"coffeeproyect/internal/service"
	"coffeeproyect/internal/store/memory"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testCoffeeID    = "c1a2b3c4-0001-0001-0001-000000000001"
	noStockCoffeeID = "c1a2b3c4-0005-0005-0005-000000000005"
	testSessionID   = "test-session-abc123"
)

func newTestCartMux() (*http.ServeMux, *service.CartService) {
	coffeeStore := memory.NewCoffeeStore()
	guestCart := memory.NewCartStore()
	userCart := memory.NewCartStore()
	cartSvc := service.NewCartService(guestCart, userCart, coffeeStore)
	cartH := NewCartHandler(cartSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /cart", cartH.GetCart)
	mux.HandleFunc("POST /cart/items", cartH.AddItem)
	mux.HandleFunc("PATCH /cart/items/{coffeeId}", cartH.SetQuantity)
	mux.HandleFunc("DELETE /cart/items/{coffeeId}", cartH.RemoveItem)
	mux.HandleFunc("DELETE /cart", cartH.ClearCart)
	return mux, cartSvc
}

func cartPost(mux *http.ServeMux, path string, body any, sessionID string) *httptest.ResponseRecorder {
	return postWithSession(mux, path, body, sessionID)
}

func postWithSession(mux *http.ServeMux, path string, body any, sessionID string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func cartGet(mux *http.ServeMux, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func cartDelete(mux *http.ServeMux, path, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func cartPatch(mux *http.ServeMux, path string, body any, sessionID string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ── GetCart ───────────────────────────────────────────────────────────────────

func TestGetCart_emptyGuestCart(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartGet(mux, testSessionID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	items := body["items"].([]any)
	if len(items) != 0 {
		t.Errorf("expected empty cart, got %d items", len(items))
	}
}

func TestGetCart_noIdentity(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartGet(mux, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── AddItem ───────────────────────────────────────────────────────────────────

func TestAddItem_success(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartPost(mux, "/cart/items", map[string]any{
		"coffee_id": testCoffeeID, "quantity": 2,
	}, testSessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}

	var cart map[string]any
	json.NewDecoder(rec.Body).Decode(&cart)
	items := cart["items"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	total := cart["total_cents"].(float64)
	if total != 2*8500000 {
		t.Errorf("expected total 17000000, got %v", total)
	}
}

func TestAddItem_accumulatesQuantity(t *testing.T) {
	mux, _ := newTestCartMux()
	cartPost(mux, "/cart/items", map[string]any{"coffee_id": testCoffeeID, "quantity": 1}, testSessionID)
	cartPost(mux, "/cart/items", map[string]any{"coffee_id": testCoffeeID, "quantity": 2}, testSessionID)

	rec := cartGet(mux, testSessionID)
	var cart map[string]any
	json.NewDecoder(rec.Body).Decode(&cart)
	items := cart["items"].([]any)
	item := items[0].(map[string]any)
	if item["quantity"].(float64) != 3 {
		t.Errorf("expected quantity 3, got %v", item["quantity"])
	}
}

func TestAddItem_coffeeNotAvailable(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartPost(mux, "/cart/items", map[string]any{
		"coffee_id": noStockCoffeeID, "quantity": 1,
	}, testSessionID)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestAddItem_coffeeNotFound(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartPost(mux, "/cart/items", map[string]any{
		"coffee_id": "00000000-0000-0000-0000-000000000000", "quantity": 1,
	}, testSessionID)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAddItem_invalidQuantity(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartPost(mux, "/cart/items", map[string]any{
		"coffee_id": testCoffeeID, "quantity": 0,
	}, testSessionID)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── SetQuantity ───────────────────────────────────────────────────────────────

func TestSetQuantity_success(t *testing.T) {
	mux, _ := newTestCartMux()
	cartPost(mux, "/cart/items", map[string]any{"coffee_id": testCoffeeID, "quantity": 3}, testSessionID)

	rec := cartPatch(mux, "/cart/items/"+testCoffeeID, map[string]any{"quantity": 5}, testSessionID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body)
	}

	var cart map[string]any
	json.NewDecoder(rec.Body).Decode(&cart)
	item := cart["items"].([]any)[0].(map[string]any)
	if item["quantity"].(float64) != 5 {
		t.Errorf("expected quantity 5, got %v", item["quantity"])
	}
}

func TestSetQuantity_itemNotInCart(t *testing.T) {
	mux, _ := newTestCartMux()
	rec := cartPatch(mux, "/cart/items/"+testCoffeeID, map[string]any{"quantity": 2}, testSessionID)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ── RemoveItem ────────────────────────────────────────────────────────────────

func TestRemoveItem_success(t *testing.T) {
	mux, _ := newTestCartMux()
	cartPost(mux, "/cart/items", map[string]any{"coffee_id": testCoffeeID, "quantity": 1}, testSessionID)
	rec := cartDelete(mux, "/cart/items/"+testCoffeeID, testSessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var cart map[string]any
	json.NewDecoder(rec.Body).Decode(&cart)
	if len(cart["items"].([]any)) != 0 {
		t.Error("expected empty cart after remove")
	}
}

// ── ClearCart ─────────────────────────────────────────────────────────────────

func TestClearCart_success(t *testing.T) {
	mux, _ := newTestCartMux()
	cartPost(mux, "/cart/items", map[string]any{"coffee_id": testCoffeeID, "quantity": 2}, testSessionID)
	rec := cartDelete(mux, "/cart", testSessionID)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
	recGet := cartGet(mux, testSessionID)
	var cart map[string]any
	json.NewDecoder(recGet.Body).Decode(&cart)
	if len(cart["items"].([]any)) != 0 {
		t.Error("expected empty cart after clear")
	}
}

