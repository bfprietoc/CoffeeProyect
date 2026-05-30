package memory

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"testing"
)

func TestGetByID_found(t *testing.T) {
	s := NewCoffeeStore()
	coffee, err := s.GetByID("c1a2b3c4-0001-0001-0001-000000000001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if coffee.Name != "El Paraíso 92" {
		t.Errorf("expected 'El Paraíso 92', got %q", coffee.Name)
	}
}

func TestGetByID_notFound(t *testing.T) {
	s := NewCoffeeStore()
	_, err := s.GetByID("does-not-exist")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestList_noFilters(t *testing.T) {
	s := NewCoffeeStore()
	coffees, err := s.List(store.CoffeeFilters{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(coffees) != 5 {
		t.Errorf("expected 5 coffees, got %d", len(coffees))
	}
}

func TestList_filterByAvailable(t *testing.T) {
	s := NewCoffeeStore()
	trueVal := true
	coffees, err := s.List(store.CoffeeFilters{Available: &trueVal})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range coffees {
		if !c.Available {
			t.Errorf("expected only available coffees, got unavailable: %s", c.Name)
		}
	}
	if len(coffees) != 4 {
		t.Errorf("expected 4 available coffees, got %d", len(coffees))
	}
}

func TestList_filterByRoastLevel(t *testing.T) {
	s := NewCoffeeStore()
	coffees, err := s.List(store.CoffeeFilters{RoastLevel: "light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range coffees {
		if c.RoastLevel != "light" {
			t.Errorf("expected roast_level=light, got %q for %s", c.RoastLevel, c.Name)
		}
	}
}

func TestList_filterByCountry(t *testing.T) {
	s := NewCoffeeStore()
	coffees, err := s.List(store.CoffeeFilters{Country: "Colombia"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(coffees) != 5 {
		t.Errorf("expected 5 colombian coffees, got %d", len(coffees))
	}
}

func TestList_pagination(t *testing.T) {
	s := NewCoffeeStore()
	page1, _ := s.List(store.CoffeeFilters{Page: 1, Limit: 2})
	page2, _ := s.List(store.CoffeeFilters{Page: 2, Limit: 2})
	page3, _ := s.List(store.CoffeeFilters{Page: 3, Limit: 2})

	if len(page1) != 2 {
		t.Errorf("page 1: expected 2, got %d", len(page1))
	}
	if len(page2) != 2 {
		t.Errorf("page 2: expected 2, got %d", len(page2))
	}
	if len(page3) != 1 {
		t.Errorf("page 3: expected 1, got %d", len(page3))
	}
	if page1[0].ID == page2[0].ID {
		t.Error("page 1 and page 2 returned the same first item")
	}
}

func TestList_pageOutOfRange(t *testing.T) {
	s := NewCoffeeStore()
	coffees, err := s.List(store.CoffeeFilters{Page: 99, Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(coffees) != 0 {
		t.Errorf("expected empty slice, got %d items", len(coffees))
	}
}
