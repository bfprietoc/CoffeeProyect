package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	memstore "coffeeproyect/internal/store/memory"
	"testing"
)

func newNotificationEnv() (*NotificationService, *captureEmail, *memstore.UserStore, *memstore.OrderStore) {
	userStore := memstore.NewUserStore()
	orderStore := memstore.NewOrderStore()
	mailer := &captureEmail{}
	svc := NewNotificationService(mailer, orderStore, userStore)
	return svc, mailer, userStore, orderStore
}

func seedUserAndOrder(t *testing.T, userStore *memstore.UserStore, orderStore *memstore.OrderStore) (domain.User, domain.Order) {
	t.Helper()
	user, _ := userStore.Create(domain.User{Name: "Brayan", Email: "brayan@test.com", PasswordHash: "x"})
	order, _ := orderStore.Create(domain.Order{
		UserID: user.ID, Status: domain.OrderStatusConfirmed,
		TotalCents: 8500000, Currency: "COP",
		ShippingStreet: "Calle 1", ShippingCity: "Bogotá", ShippingDept: "Cundi", ShippingCountry: "CO",
		Items: []domain.OrderItem{
			{CoffeeID: inStockCoffeeID, CoffeeName: "El Paraíso 92", Quantity: 1, UnitPriceCents: 8500000, SubtotalCents: 8500000},
		},
	})
	return user, order
}

func TestNotificationService_OnOrderCreated_sendsToUser(t *testing.T) {
	svc, mailer, userStore, orderStore := newNotificationEnv()
	user, order := seedUserAndOrder(t, userStore, orderStore)

	if err := svc.OnOrderCreated(order.ID, user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected email to %q, got %v", user.Email, mailer.sent)
	}
}

func TestNotificationService_OnOrderShipped_sendsToUser(t *testing.T) {
	svc, mailer, userStore, orderStore := newNotificationEnv()
	user, order := seedUserAndOrder(t, userStore, orderStore)

	tracking := "TK-001-CO"
	orderStore.UpdateStatus(order.ID, domain.OrderStatusShipped, &tracking)

	if err := svc.OnOrderShipped(order.ID, user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected shipping email to %q, got %v", user.Email, mailer.sent)
	}
}

func TestNotificationService_OnOrderDelivered_sendsToUser(t *testing.T) {
	svc, mailer, userStore, orderStore := newNotificationEnv()
	user, order := seedUserAndOrder(t, userStore, orderStore)

	if err := svc.OnOrderDelivered(order.ID, user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected delivery email to %q, got %v", user.Email, mailer.sent)
	}
}

func TestNotificationService_OnOrderCancelled_sendsToUser(t *testing.T) {
	svc, mailer, userStore, orderStore := newNotificationEnv()
	user, order := seedUserAndOrder(t, userStore, orderStore)

	if err := svc.OnOrderCancelled(order.ID, user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected cancellation email to %q, got %v", user.Email, mailer.sent)
	}
}

func TestNotificationService_OnUserRegistered_sendsWelcome(t *testing.T) {
	svc, mailer, _, _ := newNotificationEnv()

	if err := svc.OnUserRegistered("fan@test.com", "Fan"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != "fan@test.com" {
		t.Errorf("expected welcome email to fan@test.com, got %v", mailer.sent)
	}
}

func TestNotificationService_nilEmail_noop(t *testing.T) {
	svc := NewNotificationService(nil, memstore.NewOrderStore(), memstore.NewUserStore())

	// all methods must return nil and not panic when email sender is nil
	if err := svc.OnUserRegistered("x@test.com", "X"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// Integration: UserService.Register → bus → NotificationService.OnUserRegistered
func TestNotificationService_UserRegistered_viaUserService(t *testing.T) {
	userStore := memstore.NewUserStore()
	bus := event.NewBus()
	mailer := &captureEmail{}
	notifSvc := NewNotificationService(mailer, memstore.NewOrderStore(), userStore)

	bus.Subscribe(event.TopicUserRegistered, func(_ string, p any) {
		if up, ok := p.(event.UserPayload); ok {
			_ = notifSvc.OnUserRegistered(up.Email, up.Name)
		}
	})

	userSvc := NewUserService(userStore, "secret").WithPublisher(bus)
	if _, _, err := userSvc.Register("Brayan", "brayan@example.com", "password123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != "brayan@example.com" {
		t.Errorf("expected welcome email to brayan@example.com, got %v", mailer.sent)
	}
}

// Integration: CheckoutService.PlaceOrder → bus → NotificationService.OnOrderCreated
func TestNotificationService_OrderCreated_viaCheckoutService(t *testing.T) {
	coffeeStore := memstore.NewCoffeeStore()
	cartStore := memstore.NewCartStore()
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()
	bus := event.NewBus()
	mailer := &captureEmail{}
	notifSvc := NewNotificationService(mailer, orderStore, userStore)

	bus.Subscribe(event.TopicOrderCreated, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			_ = notifSvc.OnOrderCreated(op.OrderID, op.UserID)
		}
	})

	user, _ := userStore.Create(domain.User{Name: "Test", Email: "t@test.com", PasswordHash: "x"})
	addr, _ := userStore.AddAddress(domain.Address{
		UserID: user.ID, Label: "casa", Street: "Calle 1",
		City: "Bogotá", Department: "Cundi", Country: "CO",
	})
	cartStore.AddItem(user.ID, domain.CartItem{
		CoffeeID: inStockCoffeeID, CoffeeName: "El Paraíso", Quantity: 1, UnitPriceCents: 8500000,
	})

	checkoutSvc := NewCheckoutService(cartStore, coffeeStore, orderStore, userStore).
		WithPublisher(bus)
	if _, err := checkoutSvc.PlaceOrder(user.ID, addr.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected confirmation email to %q, got %v", user.Email, mailer.sent)
	}
}

// Integration: OrderService.UpdateStatus(shipped) → bus → NotificationService.OnOrderShipped
func TestNotificationService_OrderShipped_viaOrderService(t *testing.T) {
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()
	bus := event.NewBus()
	mailer := &captureEmail{}
	notifSvc := NewNotificationService(mailer, orderStore, userStore)

	bus.Subscribe(event.TopicOrderShipped, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			_ = notifSvc.OnOrderShipped(op.OrderID, op.UserID)
		}
	})

	user, _ := userStore.Create(domain.User{Name: "Fan", Email: "fan@test.com", PasswordHash: "x"})
	order, _ := orderStore.Create(domain.Order{
		UserID: user.ID, Status: domain.OrderStatusConfirmed,
		TotalCents: 8500000, Currency: "COP",
		ShippingStreet: "Calle 1", ShippingCity: "Bogotá", ShippingDept: "Cundi", ShippingCountry: "CO",
	})

	orderSvc := NewOrderService(orderStore).WithPublisher(bus)
	tracking := "TK-999"
	if _, err := orderSvc.UpdateStatus(order.ID, domain.OrderStatusShipped, &tracking); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected shipping email to %q, got %v", user.Email, mailer.sent)
	}
}

// Integration: OrderService.Cancel → bus → NotificationService.OnOrderCancelled
func TestNotificationService_OrderCancelled_viaOrderService(t *testing.T) {
	orderStore := memstore.NewOrderStore()
	userStore := memstore.NewUserStore()
	bus := event.NewBus()
	mailer := &captureEmail{}
	notifSvc := NewNotificationService(mailer, orderStore, userStore)

	bus.Subscribe(event.TopicOrderCancelled, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			_ = notifSvc.OnOrderCancelled(op.OrderID, op.UserID)
		}
	})

	user, _ := userStore.Create(domain.User{Name: "Fan", Email: "fan@test.com", PasswordHash: "x"})
	order, _ := orderStore.Create(domain.Order{
		UserID: user.ID, Status: domain.OrderStatusConfirmed,
		TotalCents: 8500000, Currency: "COP",
		ShippingStreet: "St", ShippingCity: "City", ShippingDept: "Dept", ShippingCountry: "CO",
	})

	orderSvc := NewOrderService(orderStore).WithPublisher(bus)
	if _, err := orderSvc.Cancel(order.ID, user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != user.Email {
		t.Errorf("expected cancellation email to %q, got %v", user.Email, mailer.sent)
	}
}
