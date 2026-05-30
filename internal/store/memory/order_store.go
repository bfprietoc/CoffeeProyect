package memory

import (
	"coffeeproyect/internal/domain"
	"fmt"
	"sync"
	"sync/atomic"
)

var orderSeq int64

func nextOrderID() string {
	return fmt.Sprintf("mem-order-%d", atomic.AddInt64(&orderSeq, 1))
}

func nextItemID() string {
	return fmt.Sprintf("mem-item-%d", atomic.AddInt64(&orderSeq, 1))
}

type OrderStore struct {
	mu     sync.RWMutex
	orders map[string]domain.Order // id → order
}

func NewOrderStore() *OrderStore {
	return &OrderStore{orders: make(map[string]domain.Order)}
}

// Create stores the order (no real stock deduction — service validated beforehand).
func (s *OrderStore) Create(order domain.Order) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order.ID = nextOrderID()
	for i := range order.Items {
		order.Items[i].ID = nextItemID()
		order.Items[i].SubtotalCents = order.Items[i].Quantity * order.Items[i].UnitPriceCents
	}
	s.orders[order.ID] = order
	return order, nil
}

func (s *OrderStore) GetByID(id string) (domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, nil
}

func (s *OrderStore) GetByIDAndUser(id, userID string) (domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok || o.UserID != userID {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, nil
}

func (s *OrderStore) ListByUser(userID string) ([]domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Order, 0)
	for _, o := range s.orders {
		if o.UserID == userID {
			result = append(result, o)
		}
	}
	return result, nil
}

func (s *OrderStore) ListAll(statusFilter string, limit, offset int) ([]domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Order, 0)
	for _, o := range s.orders {
		if statusFilter == "" || string(o.Status) == statusFilter {
			result = append(result, o)
		}
	}
	if offset >= len(result) {
		return []domain.Order{}, nil
	}
	end := offset + limit
	if end > len(result) || limit <= 0 {
		end = len(result)
	}
	return result[offset:end], nil
}

// Cancel sets status to cancelled (no stock restoration in memory).
func (s *OrderStore) Cancel(orderID, userID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[orderID]
	if !ok || o.UserID != userID {
		return domain.Order{}, domain.ErrNotFound
	}
	if !o.Status.IsCancellable() {
		return domain.Order{}, domain.ErrOrderNotCancellable
	}
	o.Status = domain.OrderStatusCancelled
	s.orders[orderID] = o
	return o, nil
}

func (s *OrderStore) UpdateStatus(id string, status domain.OrderStatus, trackingNumber *string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	o.Status = status
	o.TrackingNumber = trackingNumber
	s.orders[id] = o
	return o, nil
}
