package service

import (
	"coffeeproyect/internal/domain"
	memstore "coffeeproyect/internal/store/memory"
	"errors"
	"testing"
)

func setupOrderService() (*OrderService, *memstore.OrderStore, string) {
	orderStore := memstore.NewOrderStore()
	svc := NewOrderService(orderStore)

	// pre-seed an order
	order := domain.Order{
		UserID:          testUserID,
		Status:          domain.OrderStatusConfirmed,
		TotalCents:      8500000,
		Currency:        "COP",
		ShippingStreet:  "Calle 1",
		ShippingCity:    "Bogotá",
		ShippingDept:    "Cundi",
		ShippingCountry: "CO",
		Items: []domain.OrderItem{
			{CoffeeID: availCoffeeID, CoffeeName: "El Paraíso 92", Quantity: 1, UnitPriceCents: 8500000},
		},
	}
	created, _ := orderStore.Create(order)
	return svc, orderStore, created.ID
}

func TestOrderService_GetByID_success(t *testing.T) {
	svc, _, orderID := setupOrderService()

	o, err := svc.GetByID(orderID, testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.ID != orderID {
		t.Errorf("got wrong order ID: %s", o.ID)
	}
}

func TestOrderService_GetByID_wrongUser(t *testing.T) {
	svc, _, orderID := setupOrderService()

	_, err := svc.GetByID(orderID, "other-user")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestOrderService_ListByUser(t *testing.T) {
	svc, _, _ := setupOrderService()

	orders, err := svc.ListByUser(testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 1 {
		t.Errorf("expected 1 order, got %d", len(orders))
	}
}

func TestOrderService_Cancel_success(t *testing.T) {
	svc, _, orderID := setupOrderService()

	o, err := svc.Cancel(orderID, testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != domain.OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", o.Status)
	}
}

func TestOrderService_Cancel_wrongUser(t *testing.T) {
	svc, _, orderID := setupOrderService()

	_, err := svc.Cancel(orderID, "other-user")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestOrderService_Cancel_notCancellable(t *testing.T) {
	svc, orderStore, orderID := setupOrderService()

	// deliver the order first
	orderStore.UpdateStatus(orderID, domain.OrderStatusDelivered, nil)

	_, err := svc.Cancel(orderID, testUserID)
	if !errors.Is(err, domain.ErrOrderNotCancellable) {
		t.Errorf("expected ErrOrderNotCancellable, got %v", err)
	}
}
