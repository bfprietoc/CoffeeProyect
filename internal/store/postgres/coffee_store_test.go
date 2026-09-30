//go:build integration

package postgres

import (
	"coffeeproyect/internal/db"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"database/sql"
	"errors"
	"os"
	"testing"
)

func setupDB(t *testing.T) *CoffeeStore {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	database, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.RunMigrations(database, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	applySeed(t, database)
	t.Cleanup(func() { database.Close() })

	return NewCoffeeStore(database)
}

func applySeed(t *testing.T, database *sql.DB) {
	t.Helper()
	seed, err := os.ReadFile("../../../migrations/seed.sql")
	if err != nil {
		t.Fatalf("read seed.sql: %v", err)
	}
	if _, err := database.Exec(string(seed)); err != nil {
		t.Fatalf("apply seed: %v", err)
	}
}

func TestPostgresCoffeeStore_GetByID_found(t *testing.T) {
	s := setupDB(t)

	coffee, err := s.GetByID("c1a2b3c4-0001-0001-0001-000000000001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coffee.Name != "El Paraíso 92" {
		t.Errorf("expected 'El Paraíso 92', got %q", coffee.Name)
	}
	if coffee.Farm.Country != "Colombia" {
		t.Errorf("expected farm country Colombia, got %q", coffee.Farm.Country)
	}
	if len(coffee.TastingNotes) == 0 {
		t.Error("expected tasting notes, got empty slice")
	}
}

func TestPostgresCoffeeStore_GetByID_notFound(t *testing.T) {
	s := setupDB(t)

	_, err := s.GetByID("00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestPostgresCoffeeStore_List_noFilters(t *testing.T) {
	s := setupDB(t)

	coffees, err := s.List(store.CoffeeFilters{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(coffees) < 5 {
		t.Errorf("expected at least 5 coffees, got %d", len(coffees))
	}
}

func TestPostgresCoffeeStore_List_filterAvailable(t *testing.T) {
	s := setupDB(t)

	trueVal := true
	coffees, err := s.List(store.CoffeeFilters{Available: &trueVal})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range coffees {
		if !c.Available {
			t.Errorf("got unavailable coffee in available=true filter: %s", c.Name)
		}
	}
}

func TestPostgresCoffeeStore_List_filterRoastLevel(t *testing.T) {
	s := setupDB(t)

	coffees, err := s.List(store.CoffeeFilters{RoastLevel: "light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range coffees {
		if c.RoastLevel != "light" {
			t.Errorf("expected roast_level=light, got %q", c.RoastLevel)
		}
	}
}

func TestPostgresCoffeeStore_List_pagination(t *testing.T) {
	s := setupDB(t)

	page1, _ := s.List(store.CoffeeFilters{Page: 1, Limit: 2})
	page2, _ := s.List(store.CoffeeFilters{Page: 2, Limit: 2})

	if len(page1) != 2 {
		t.Errorf("page 1: expected 2, got %d", len(page1))
	}
	if len(page2) < 2 {
		t.Errorf("page 2: expected at least 2, got %d", len(page2))
	}
	if page1[0].ID == page2[0].ID {
		t.Error("page 1 and page 2 returned the same item")
	}
}
