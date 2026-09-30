package memory

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"fmt"
	"sync"
	"time"
)

type StockStore struct {
	mu        sync.Mutex
	stock     map[string]int
	movements map[string][]domain.StockMovement
}

func NewStockStore() *StockStore {
	return &StockStore{
		stock:     make(map[string]int),
		movements: make(map[string][]domain.StockMovement),
	}
}

func (s *StockStore) Adjust(coffeeID string, delta int, note string) (store.StockAdjustResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	old := s.stock[coffeeID]
	result := old + delta
	if result < 0 {
		return store.StockAdjustResult{}, fmt.Errorf("%w: cannot reduce below 0", domain.ErrInsufficientStock)
	}
	s.stock[coffeeID] = result
	s.movements[coffeeID] = append(s.movements[coffeeID], domain.StockMovement{
		ID: nextOrderID(), CoffeeID: coffeeID, Delta: delta,
		ResultStock: result, Note: note, CreatedAt: time.Now(),
	})
	return store.StockAdjustResult{OldStock: old, ResultStock: result}, nil
}

func (s *StockStore) GetStock(coffeeID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stock[coffeeID], nil
}

func (s *StockStore) GetMovements(coffeeID string) ([]domain.StockMovement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mvs := s.movements[coffeeID]
	if mvs == nil {
		return []domain.StockMovement{}, nil
	}
	return mvs, nil
}
