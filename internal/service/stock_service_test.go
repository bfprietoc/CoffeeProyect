package service

import (
	memorycache "coffeeproyect/internal/cache/memory"
	"coffeeproyect/internal/event"
	memstore "coffeeproyect/internal/store/memory"
	"testing"
)

func newStockSvc() (*StockService, *event.Bus, *memstore.StockStore) {
	stockStore := memstore.NewStockStore()
	bus := event.NewBus()
	coffeeStore := memstore.NewCoffeeStore()
	coffeeSvc := NewCoffeeService(coffeeStore).WithCache(memorycache.NewCoffeeCache())
	svc := NewStockService(stockStore, coffeeSvc, bus)
	return svc, bus, stockStore
}

func TestStockService_Adjust_publishesReplenished(t *testing.T) {
	svc, bus, _ := newStockSvc()

	published := false
	bus.Subscribe(event.TopicStockReplenished, func(_ string, _ any) {
		published = true
	})

	if _, err := svc.Adjust("some-coffee", 10, "reposición"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !published {
		t.Error("expected stock.replenished event to be published")
	}
}

func TestStockService_Adjust_publishesDepleted(t *testing.T) {
	svc, bus, stockStore := newStockSvc()

	// seed 5 bags
	stockStore.Adjust("some-coffee", 5, "seed")

	depleted := false
	bus.Subscribe(event.TopicStockDepleted, func(_ string, _ any) {
		depleted = true
	})

	if _, err := svc.Adjust("some-coffee", -5, "sold out"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !depleted {
		t.Error("expected stock.depleted event when stock reaches 0")
	}
}

func TestStockService_Adjust_publishesRestored(t *testing.T) {
	svc, bus, _ := newStockSvc()
	// stock starts at 0 (empty), adding stock fires stock.restored

	restored := false
	bus.Subscribe(event.TopicStockRestored, func(_ string, _ any) {
		restored = true
	})

	if _, err := svc.Adjust("some-coffee", 5, "restocked"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !restored {
		t.Error("expected stock.restored when stock goes from 0 to >0")
	}
}

func TestStockService_Adjust_invalidatesCoffeeCache(t *testing.T) {
	coffeeStore := memstore.NewCoffeeStore()
	counting := &countingCoffeeStore{inner: coffeeStore}
	coffeeSvc := NewCoffeeService(counting).WithCache(memorycache.NewCoffeeCache())
	stockStore := memstore.NewStockStore()
	svc := NewStockService(stockStore, coffeeSvc, nil)

	const id = "c1a2b3c4-0001-0001-0001-000000000001"

	// prime cache
	coffeeSvc.GetByID(id)
	hitsBefore := counting.getByIDHits

	// adjust stock → should invalidate cache
	svc.Adjust(id, 5, "test")

	// next fetch must hit store
	coffeeSvc.GetByID(id)
	if counting.getByIDHits != hitsBefore+1 {
		t.Errorf("expected cache to be invalidated after stock adjust, store hits: %d", counting.getByIDHits)
	}
}
