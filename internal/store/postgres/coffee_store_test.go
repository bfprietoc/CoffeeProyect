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
	s, _ := setupDBWithRaw(t)
	return s
}

// setupDBWithRaw es lo mismo que setupDB pero expone el *sql.DB para tests
// que necesitan insertar/inspeccionar filas directamente.
func setupDBWithRaw(t *testing.T) (*CoffeeStore, *sql.DB) {
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

	return NewCoffeeStore(database), database
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

func TestPostgresCoffeeStore_GetByID_invalidUUID(t *testing.T) {
	s := setupDB(t)
	_, err := s.GetByID("no-es-un-uuid")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for malformed uuid, got: %v", err)
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

func newTestCoffee() domain.Coffee {
	return domain.Coffee{
		Name: "Delete Test", Process: "washed", RoastLevel: "light",
		TastingNotes: []string{}, Description: "",
		Producer:     domain.Producer{Name: "Productor Test"},
		Farm:         domain.Farm{Name: "Finca Test", Country: "CO"},
		BagSizeGrams: 250, PriceCents: 100000, Currency: "COP",
		Available: true, StockBags: 5,
	}
}

// cleanupCoffee borra el café y las filas que Create genera (finca/productor),
// además de cualquier rastro en order_items/stock_movements.
func cleanupCoffee(t *testing.T, raw *sql.DB, id string) {
	t.Helper()
	var producerID, farmID string
	err := raw.QueryRow(
		`SELECT f.producer_id, f.id FROM coffees c JOIN farms f ON c.farm_id = f.id WHERE c.id = $1`, id,
	).Scan(&producerID, &farmID)
	if err != nil {
		return // ya no existe
	}
	for _, q := range []string{
		"DELETE FROM order_items WHERE coffee_id = $1",
		"DELETE FROM stock_movements WHERE coffee_id = $1",
		"DELETE FROM cart_items WHERE coffee_id = $1",
		"DELETE FROM waitlist WHERE coffee_id = $1",
	} {
		if _, err := raw.Exec(q, id); err != nil {
			t.Logf("cleanup %s: %v", q, err)
		}
	}
	if _, err := raw.Exec(`DELETE FROM coffees WHERE id = $1`, id); err != nil {
		t.Logf("cleanup coffees: %v", err)
	}
	if _, err := raw.Exec(`DELETE FROM farms WHERE id = $1`, farmID); err != nil {
		t.Logf("cleanup farms: %v", err)
	}
	if _, err := raw.Exec(`DELETE FROM producers WHERE id = $1`, producerID); err != nil {
		t.Logf("cleanup producers: %v", err)
	}
}

// Regresión: stock_movements tiene FK NO ACTION a coffees; borrar un café con
// movimientos debía fallar con 23503 (y antes, además, crasheaba el servidor
// por la recursión de writeInternalError).
func TestPostgresCoffeeStore_Delete_cleansUpStockMovements(t *testing.T) {
	s, raw := setupDBWithRaw(t)

	coffee, err := s.Create(newTestCoffee())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { cleanupCoffee(t, raw, coffee.ID) })

	if _, err := raw.Exec(
		`INSERT INTO stock_movements (coffee_id, delta, result_stock, note) VALUES ($1, 2, 7, 'test')`,
		coffee.ID,
	); err != nil {
		t.Fatalf("insert movement: %v", err)
	}

	if err := s.Delete(coffee.ID); err != nil {
		t.Fatalf("expected delete to succeed, got: %v", err)
	}

	var count int
	if err := raw.QueryRow(`SELECT count(*) FROM coffees WHERE id = $1`, coffee.ID).Scan(&count); err != nil {
		t.Fatalf("count coffees: %v", err)
	}
	if count != 0 {
		t.Errorf("coffee should be gone, got %d rows", count)
	}
	if err := raw.QueryRow(`SELECT count(*) FROM stock_movements WHERE coffee_id = $1`, coffee.ID).Scan(&count); err != nil {
		t.Fatalf("count movements: %v", err)
	}
	if count != 0 {
		t.Errorf("movements should be gone, got %d rows", count)
	}
}

// Un café que ya fue vendido no se puede borrar: destruir order_items
// corrompería el historial de órdenes.
func TestPostgresCoffeeStore_Delete_blockedByOrders(t *testing.T) {
	s, raw := setupDBWithRaw(t)

	coffee, err := s.Create(newTestCoffee())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { cleanupCoffee(t, raw, coffee.ID) })

	var userID, orderID string
	if err := raw.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Delete Test', 'delete-test@example.com', 'x') RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := raw.QueryRow(
		`INSERT INTO orders (user_id, shipping_street, shipping_city, shipping_department, total_cents)
		 VALUES ($1, 'Cra 1 #2-3', 'Medellín', 'Antioquia', 100000) RETURNING id`, userID,
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	if _, err := raw.Exec(
		`INSERT INTO order_items (order_id, coffee_id, coffee_name, quantity, unit_price_cents)
		 VALUES ($1, $2, 'Delete Test', 1, 100000)`, orderID, coffee.ID,
	); err != nil {
		t.Fatalf("insert order item: %v", err)
	}
	t.Cleanup(func() {
		raw.Exec(`DELETE FROM orders WHERE id = $1`, orderID) // cascada a order_items
		raw.Exec(`DELETE FROM users WHERE id = $1`, userID)
	})

	err = s.Delete(coffee.ID)
	if !errors.Is(err, domain.ErrCoffeeHasOrders) {
		t.Fatalf("expected ErrCoffeeHasOrders, got: %v", err)
	}

	var count int
	if err := raw.QueryRow(`SELECT count(*) FROM coffees WHERE id = $1`, coffee.ID).Scan(&count); err != nil {
		t.Fatalf("count coffees: %v", err)
	}
	if count != 1 {
		t.Errorf("coffee must not be deleted, got %d rows", count)
	}
}
