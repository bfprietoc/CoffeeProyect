package postgres

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"database/sql"
	"errors"
	"fmt"
)

type StockStore struct {
	db *sql.DB
}

func NewStockStore(db *sql.DB) *StockStore {
	return &StockStore{db: db}
}

func (s *StockStore) Adjust(coffeeID string, delta int, note string) (store.StockAdjustResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return store.StockAdjustResult{}, err
	}
	defer tx.Rollback()

	var old int
	err = tx.QueryRow(
		`SELECT stock_bags FROM coffees WHERE id = $1 FOR UPDATE`, coffeeID,
	).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) {
		return store.StockAdjustResult{}, domain.ErrNotFound
	}
	if err != nil {
		return store.StockAdjustResult{}, fmt.Errorf("lock coffee: %w", err)
	}

	result := old + delta
	if result < 0 {
		return store.StockAdjustResult{}, fmt.Errorf("%w: would go negative", domain.ErrInsufficientStock)
	}

	if _, err := tx.Exec(
		`UPDATE coffees SET stock_bags = $1 WHERE id = $2`, result, coffeeID,
	); err != nil {
		return store.StockAdjustResult{}, fmt.Errorf("update stock: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO stock_movements (coffee_id, delta, result_stock, note) VALUES ($1,$2,$3,$4)`,
		coffeeID, delta, result, note,
	); err != nil {
		return store.StockAdjustResult{}, fmt.Errorf("record movement: %w", err)
	}

	return store.StockAdjustResult{OldStock: old, ResultStock: result}, tx.Commit()
}

func (s *StockStore) GetStock(coffeeID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT stock_bags FROM coffees WHERE id = $1`, coffeeID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	return n, err
}

func (s *StockStore) GetMovements(coffeeID string) ([]domain.StockMovement, error) {
	rows, err := s.db.Query(`
		SELECT id, coffee_id, delta, result_stock, note, created_at
		FROM stock_movements WHERE coffee_id = $1 ORDER BY created_at DESC`, coffeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mvs := make([]domain.StockMovement, 0)
	for rows.Next() {
		var m domain.StockMovement
		if err := rows.Scan(&m.ID, &m.CoffeeID, &m.Delta, &m.ResultStock, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		mvs = append(mvs, m)
	}
	return mvs, rows.Err()
}
