package routes

import (
	"coffeeproyect/internal/handler"
	"net/http"
)

type Deps struct {
	Health   *handler.HealthHandler
	Coffee   *handler.CoffeeHandler
	Auth     *handler.AuthHandler
	User     *handler.UserHandler
	Cart     *handler.CartHandler
	Checkout *handler.CheckoutHandler
	Order    *handler.OrderHandler
	Admin    *handler.AdminHandler
	Waitlist *handler.WaitlistHandler
}

func Register(
	mux *http.ServeMux,
	d Deps,
	requireAuth func(http.Handler) http.Handler,
	optionalAuth func(http.Handler) http.Handler,
	requireAdmin func(http.Handler) http.Handler,
) {
	// public — sin autenticación
	mux.HandleFunc("GET /health", d.Health.Health)
	mux.HandleFunc("GET /coffees", d.Coffee.List)
	mux.HandleFunc("GET /coffees/{id}", d.Coffee.GetByID)
	mux.HandleFunc("POST /auth/register", d.Auth.Register)
	mux.HandleFunc("POST /auth/login", d.Auth.Login)
	mux.HandleFunc("POST /auth/refresh", d.Auth.Refresh)

	// carrito — funciona para invitados (X-Session-ID) y usuarios (JWT)
	// optionalAuth inyecta userID en context si hay token válido, sin bloquear si no lo hay
	mux.Handle("GET /cart", optionalAuth(http.HandlerFunc(d.Cart.GetCart)))
	mux.Handle("POST /cart/items", optionalAuth(http.HandlerFunc(d.Cart.AddItem)))
	mux.Handle("PATCH /cart/items/{coffeeId}", optionalAuth(http.HandlerFunc(d.Cart.SetQuantity)))
	mux.Handle("DELETE /cart/items/{coffeeId}", optionalAuth(http.HandlerFunc(d.Cart.RemoveItem)))
	mux.Handle("DELETE /cart", optionalAuth(http.HandlerFunc(d.Cart.ClearCart)))

	// usuarios autenticados — requieren JWT válido
	mux.Handle("GET /users/me", requireAuth(http.HandlerFunc(d.User.GetProfile)))
	mux.Handle("PUT /users/me", requireAuth(http.HandlerFunc(d.User.UpdateProfile)))
	mux.Handle("GET /users/me/addresses", requireAuth(http.HandlerFunc(d.User.GetAddresses)))
	mux.Handle("POST /users/me/addresses", requireAuth(http.HandlerFunc(d.User.AddAddress)))
	mux.Handle("DELETE /users/me/addresses/{id}", requireAuth(http.HandlerFunc(d.User.DeleteAddress)))

	// checkout y órdenes
	mux.Handle("GET /checkout/validate", requireAuth(http.HandlerFunc(d.Checkout.Validate)))
	mux.Handle("POST /orders", requireAuth(http.HandlerFunc(d.Checkout.PlaceOrder)))
	mux.Handle("GET /orders", requireAuth(http.HandlerFunc(d.Order.List)))
	mux.Handle("GET /orders/{id}", requireAuth(http.HandlerFunc(d.Order.GetByID)))
	mux.Handle("POST /orders/{id}/cancel", requireAuth(http.HandlerFunc(d.Order.Cancel)))

	// waitlist — optionalAuth para asociar con usuario si hay token
	mux.Handle("POST /waitlist", optionalAuth(http.HandlerFunc(d.Waitlist.Subscribe)))

	// admin — requieren X-Api-Key
	mux.Handle("POST /admin/coffees", requireAdmin(http.HandlerFunc(d.Admin.CreateCoffee)))
	mux.Handle("PUT /admin/coffees/{id}", requireAdmin(http.HandlerFunc(d.Admin.UpdateCoffee)))
	mux.Handle("DELETE /admin/coffees/{id}", requireAdmin(http.HandlerFunc(d.Admin.DeleteCoffee)))
	mux.Handle("GET /admin/coffees/{id}/stock", requireAdmin(http.HandlerFunc(d.Admin.GetStock)))
	mux.Handle("PATCH /admin/coffees/{id}/stock", requireAdmin(http.HandlerFunc(d.Admin.AdjustStock)))
	mux.Handle("GET /admin/orders", requireAdmin(http.HandlerFunc(d.Admin.ListOrders)))
	mux.Handle("PATCH /admin/orders/{id}/status", requireAdmin(http.HandlerFunc(d.Admin.UpdateOrderStatus)))
}
