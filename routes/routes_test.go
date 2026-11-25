package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"coffeeproyect/internal/handler"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/service"
	memstore "coffeeproyect/internal/store/memory"
)

// testMux builds the exact production mux (routes.Register + middleware)
// with in-memory stores. It guards against routes that exist in the code
// but are never registered (missing mux.Handle), which the handler-level
// tests can't see.
func testMux(t *testing.T) *http.ServeMux {
	t.Helper()

	coffeeStore := memstore.NewCoffeeStore()
	userStore := memstore.NewUserStore()
	guestCartStore := memstore.NewCartStore()
	userCartStore := memstore.NewCartStore()
	orderStore := memstore.NewOrderStore()
	stockStore := memstore.NewStockStore()
	waitlistStore := memstore.NewWaitlistStore()

	userSvc := service.NewUserService(userStore, "route-test-secret")
	coffeeSvc := service.NewCoffeeService(coffeeStore)
	cartSvc := service.NewCartService(guestCartStore, userCartStore, coffeeStore)
	orderSvc := service.NewOrderService(orderStore)
	checkoutSvc := service.NewCheckoutService(guestCartStore, coffeeStore, orderStore, userStore)
	stockSvc := service.NewStockService(stockStore, coffeeSvc, nil)
	waitlistSvc := service.NewWaitlistService(waitlistStore, coffeeStore, nil, nil)
	paymentSvc := service.NewPaymentService(orderStore, userStore, service.PaymentConfig{}) // disabled on purpose

	deps := Deps{
		Health:   handler.NewHealthHandler(),
		Coffee:   handler.NewCoffeeHandler(coffeeSvc),
		Auth:     handler.NewAuthHandler(userSvc, cartSvc),
		User:     handler.NewUserHandler(userSvc),
		Cart:     handler.NewCartHandler(cartSvc),
		Checkout: handler.NewCheckoutHandler(checkoutSvc),
		Order:    handler.NewOrderHandler(orderSvc),
		Payment:  handler.NewPaymentHandler(paymentSvc),
		Admin:    handler.NewAdminHandler(stockSvc, orderSvc),
		Waitlist: handler.NewWaitlistHandler(waitlistSvc),
	}

	mux := http.NewServeMux()
	Register(mux, deps,
		middleware.RequireAuth("route-test-secret"),
		middleware.OptionalAuth("route-test-secret"),
		middleware.RequireAdmin("route-test-admin-key"),
	)
	return mux
}

func TestPaymentRoutesAreRegistered(t *testing.T) {
	mux := testMux(t)

	t.Run("payment endpoint requires auth (401, not 404)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orders/whatever/payment", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatal("POST /orders/{id}/payment is not registered (404)")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("webhook endpoint exists (503 disabled, not 404)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/wompi", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatal("POST /webhooks/wompi is not registered (404)")
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 (payments disabled), got %d (%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHealthRouteIsRegistered(t *testing.T) {
	mux := testMux(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d", rec.Code)
	}
}
