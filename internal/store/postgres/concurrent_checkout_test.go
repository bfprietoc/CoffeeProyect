//go:build integration

package postgres

import (
	"coffeeproyect/internal/db"
	"coffeeproyect/internal/domain"
	"errors"
	"os"
	"sync"
	"testing"
)

// TestConcurrentCheckout_oneSucceeds verifies that when two goroutines attempt to purchase
// the last available bag simultaneously, exactly one succeeds and the other gets
// ErrInsufficientStock — guaranteed by the FOR UPDATE lock in OrderStore.Create.
func TestConcurrentCheckout_oneSucceeds(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	database, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	applySeed(t, database)

	// Use a seeded coffee and set its stock to exactly 1 for this test.
	const coffeeID = "c1a2b3c4-0001-0001-0001-000000000001"
	var origStock int
	database.QueryRow(`SELECT stock_bags FROM coffees WHERE id=$1`, coffeeID).Scan(&origStock)
	if _, err := database.Exec(`UPDATE coffees SET stock_bags=1 WHERE id=$1`, coffeeID); err != nil {
		t.Fatalf("set stock: %v", err)
	}

	// Create a test user.
	if _, err := database.Exec(
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) ON CONFLICT (email) DO NOTHING`,
		"Concurrent Test", "concurrent-checkout@test.com", "$2b$12$fakehash",
	); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	var testUserID string
	if err := database.QueryRow(
		`SELECT id FROM users WHERE email=$1`, "concurrent-checkout@test.com",
	).Scan(&testUserID); err != nil {
		t.Fatalf("get test user id: %v", err)
	}

	t.Cleanup(func() {
		// order_items cascade from orders; delete orders first, then restore state.
		database.Exec(`DELETE FROM orders WHERE user_id=$1`, testUserID)
		database.Exec(`UPDATE coffees SET stock_bags=$1 WHERE id=$2`, origStock, coffeeID)
		database.Exec(`DELETE FROM users WHERE id=$1`, testUserID)
	})

	orderStore := NewOrderStore(database)

	order := domain.Order{
		UserID: testUserID,
		Items: []domain.OrderItem{
			{CoffeeID: coffeeID, CoffeeName: "El Paraíso 92", Quantity: 1, UnitPriceCents: 8500000},
		},
		ShippingStreet:  "Calle 123",
		ShippingCity:    "Bogotá",
		ShippingDept:    "Cundinamarca",
		ShippingCountry: "CO",
		TotalCents:      8500000,
		Currency:        "COP",
		Status:          domain.OrderStatusConfirmed,
	}

	errs := make([]error, 2)
	var wg sync.WaitGroup

	// close(start) broadcasts to both goroutines simultaneously.
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, errs[idx] = orderStore.Create(order)
		}(i)
	}
	close(start)
	wg.Wait()

	successes, insufficientStock := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrInsufficientStock) {
			insufficientStock++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if successes != 1 || insufficientStock != 1 {
		t.Errorf("expected 1 success + 1 ErrInsufficientStock, got %d + %d", successes, insufficientStock)
	}

	// Stock must be 0 — only one order went through.
	var finalStock int
	database.QueryRow(`SELECT stock_bags FROM coffees WHERE id=$1`, coffeeID).Scan(&finalStock)
	if finalStock != 0 {
		t.Errorf("expected stock=0, got %d", finalStock)
	}
}
