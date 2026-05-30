package service

import (
	"coffeeproyect/internal/domain"
	memstore "coffeeproyect/internal/store/memory"
	"errors"
	"testing"
)

const (
	testUserID    = "user-001"
	testAddressID = "addr-001"
	availCoffeeID = "c1a2b3c4-0001-0001-0001-000000000001"
	noStockID     = "c1a2b3c4-0005-0005-0005-000000000005"
)

func newCheckoutSvc() (*CheckoutService, *memstore.CartStore, *memstore.UserStore) {
	coffeeStore := memstore.NewCoffeeStore()
	cartStore := memstore.NewCartStore()
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()

	// register a user so GetAddresses works
	u, _ := userStore.Create(domain.User{
		Name: "Test", Email: "t@test.com", PasswordHash: "x",
	})
	addr, _ := userStore.AddAddress(domain.Address{
		UserID: u.ID, Label: "casa", Street: "Calle 1",
		City: "Bogotá", Department: "Cundi", Country: "CO",
	})
	// make the ID predictable for tests
	_ = addr

	svc := NewCheckoutService(cartStore, coffeeStore, orderStore, userStore)
	return svc, cartStore, userStore
}

// resolveAddress returns the first address ID for the user.
func resolveFirstAddress(userStore *memstore.UserStore, userID string) string {
	addrs, _ := userStore.GetAddresses(userID)
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0].ID
}

// resolveUserID returns the ID of the first (and only) user in the store.
func resolveUserID(userStore *memstore.UserStore) string {
	u, _ := userStore.GetByEmail("t@test.com")
	return u.ID
}

func TestCheckout_success(t *testing.T) {
	svc, cartStore, userStore := newCheckoutSvc()
	userID := resolveUserID(userStore)
	addressID := resolveFirstAddress(userStore, userID)

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: availCoffeeID, CoffeeName: "El Paraíso 92",
		Quantity: 2, UnitPriceCents: 8500000,
	})

	order, err := svc.PlaceOrder(userID, addressID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.ID == "" {
		t.Error("expected order ID to be set")
	}
	if len(order.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(order.Items))
	}
	if order.TotalCents != 2*8500000 {
		t.Errorf("expected total %d, got %d", 2*8500000, order.TotalCents)
	}

	// cart should be cleared
	cartItems, _ := cartStore.GetItems(userID)
	if len(cartItems) != 0 {
		t.Error("cart should be empty after checkout")
	}
}

func TestCheckout_emptyCart(t *testing.T) {
	svc, _, userStore := newCheckoutSvc()
	userID := resolveUserID(userStore)
	addressID := resolveFirstAddress(userStore, userID)

	_, err := svc.PlaceOrder(userID, addressID)
	if !errors.Is(err, domain.ErrCartEmpty) {
		t.Errorf("expected ErrCartEmpty, got %v", err)
	}
}

func TestCheckout_insufficientStock(t *testing.T) {
	svc, cartStore, userStore := newCheckoutSvc()
	userID := resolveUserID(userStore)
	addressID := resolveFirstAddress(userStore, userID)

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: noStockID, CoffeeName: "Dark Roast",
		Quantity: 1, UnitPriceCents: 4800000,
	})

	_, err := svc.PlaceOrder(userID, addressID)
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Errorf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestCheckout_addressNotFound(t *testing.T) {
	svc, cartStore, userStore := newCheckoutSvc()
	userID := resolveUserID(userStore)

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: availCoffeeID, Quantity: 1, UnitPriceCents: 8500000,
	})

	_, err := svc.PlaceOrder(userID, "non-existent-address")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing address, got %v", err)
	}
}

func TestValidate_allAvailable(t *testing.T) {
	svc, cartStore, userStore := newCheckoutSvc()
	userID := resolveUserID(userStore)

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: availCoffeeID, Quantity: 1, UnitPriceCents: 8500000,
	})

	result, err := svc.Validate(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CanCheckout {
		t.Error("expected CanCheckout = true")
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestValidate_unavailableItem(t *testing.T) {
	svc, cartStore, userStore := newCheckoutSvc()
	userID := resolveUserID(userStore)

	cartStore.AddItem(userID, domain.CartItem{
		CoffeeID: noStockID, Quantity: 1, UnitPriceCents: 4800000,
	})

	result, err := svc.Validate(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CanCheckout {
		t.Error("expected CanCheckout = false for unavailable item")
	}
}
