package memory

import (
	"coffeeproyect/internal/domain"
	"sync"
)

type CartStore struct {
	mu    sync.RWMutex
	carts map[string][]domain.CartItem // cartID -> items
}

func NewCartStore() *CartStore {
	return &CartStore{carts: make(map[string][]domain.CartItem)}
}

func (s *CartStore) GetItems(cartID string) ([]domain.CartItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.carts[cartID]
	if items == nil {
		return []domain.CartItem{}, nil
	}
	return items, nil
}

func (s *CartStore) AddItem(cartID string, item domain.CartItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, it := range s.carts[cartID] {
		if it.CoffeeID == item.CoffeeID {
			s.carts[cartID][i].Quantity += item.Quantity
			s.carts[cartID][i].UnitPriceCents = item.UnitPriceCents
			return nil
		}
	}
	s.carts[cartID] = append(s.carts[cartID], item)
	return nil
}

func (s *CartStore) SetQuantity(cartID, coffeeID string, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, it := range s.carts[cartID] {
		if it.CoffeeID == coffeeID {
			s.carts[cartID][i].Quantity = quantity
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *CartStore) RemoveItem(cartID, coffeeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.carts[cartID]
	for i, it := range items {
		if it.CoffeeID == coffeeID {
			s.carts[cartID] = append(items[:i], items[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *CartStore) Clear(cartID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.carts, cartID)
	return nil
}
