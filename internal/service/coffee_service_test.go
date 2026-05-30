package service

import (
	memorycache "coffeeproyect/internal/cache/memory"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	memorystore "coffeeproyect/internal/store/memory"
	"testing"
)

// countingCoffeeStore wraps a real store and tracks call counts.
type countingCoffeeStore struct {
	inner       store.CoffeeStore
	getByIDHits int
	listHits    int
}

func (c *countingCoffeeStore) GetByID(id string) (domain.Coffee, error) {
	c.getByIDHits++
	return c.inner.GetByID(id)
}

func (c *countingCoffeeStore) List(f store.CoffeeFilters) ([]domain.Coffee, error) {
	c.listHits++
	return c.inner.List(f)
}

func (c *countingCoffeeStore) Create(coffee domain.Coffee) (domain.Coffee, error) {
	return c.inner.Create(coffee)
}

func (c *countingCoffeeStore) Update(coffee domain.Coffee) (domain.Coffee, error) {
	return c.inner.Update(coffee)
}

func (c *countingCoffeeStore) Delete(id string) error {
	return c.inner.Delete(id)
}

func TestCoffeeService_GetByID_cacheMissThenHit(t *testing.T) {
	counting := &countingCoffeeStore{inner: memorystore.NewCoffeeStore()}
	svc := NewCoffeeService(counting).WithCache(memorycache.NewCoffeeCache())

	const id = "c1a2b3c4-0001-0001-0001-000000000001"

	c1, err := svc.GetByID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counting.getByIDHits != 1 {
		t.Errorf("expected 1 store hit on cache miss, got %d", counting.getByIDHits)
	}

	c2, err := svc.GetByID(id)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if counting.getByIDHits != 1 {
		t.Errorf("expected still 1 store hit after cache hit, got %d", counting.getByIDHits)
	}
	if c1.ID != c2.ID {
		t.Errorf("cached coffee differs: %s vs %s", c1.ID, c2.ID)
	}
}

func TestCoffeeService_List_cacheMissThenHit(t *testing.T) {
	counting := &countingCoffeeStore{inner: memorystore.NewCoffeeStore()}
	svc := NewCoffeeService(counting).WithCache(memorycache.NewCoffeeCache())

	filters := store.CoffeeFilters{RoastLevel: "light"}

	list1, err := svc.List(filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counting.listHits != 1 {
		t.Errorf("expected 1 store hit on cache miss, got %d", counting.listHits)
	}

	list2, err := svc.List(filters)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if counting.listHits != 1 {
		t.Errorf("expected still 1 store hit after cache hit, got %d", counting.listHits)
	}
	if len(list1) != len(list2) {
		t.Errorf("list lengths differ: %d vs %d", len(list1), len(list2))
	}
}

func TestCoffeeService_InvalidateProduct_forcesStoreMiss(t *testing.T) {
	counting := &countingCoffeeStore{inner: memorystore.NewCoffeeStore()}
	svc := NewCoffeeService(counting).WithCache(memorycache.NewCoffeeCache())

	const id = "c1a2b3c4-0001-0001-0001-000000000001"

	svc.GetByID(id)
	if counting.getByIDHits != 1 {
		t.Fatalf("expected 1 store hit after priming")
	}

	svc.InvalidateProduct(id)

	svc.GetByID(id)
	if counting.getByIDHits != 2 {
		t.Errorf("expected 2 store hits after invalidation, got %d", counting.getByIDHits)
	}
}

func TestCoffeeService_noCache_stillWorks(t *testing.T) {
	svc := NewCoffeeService(memorystore.NewCoffeeStore())

	_, err := svc.GetByID("c1a2b3c4-0001-0001-0001-000000000001")
	if err != nil {
		t.Errorf("unexpected error without cache: %v", err)
	}

	_, err = svc.List(store.CoffeeFilters{})
	if err != nil {
		t.Errorf("unexpected error listing without cache: %v", err)
	}
}
