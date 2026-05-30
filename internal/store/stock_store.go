package store

import "coffeeproyect/internal/domain"

type StockAdjustResult struct {
	OldStock    int
	ResultStock int
}

type StockStore interface {
	// Adjust atomically updates stock_bags by delta and records the movement.
	// Returns the old and new stock values.
	Adjust(coffeeID string, delta int, note string) (StockAdjustResult, error)
	GetStock(coffeeID string) (int, error)
	GetMovements(coffeeID string) ([]domain.StockMovement, error)
}
