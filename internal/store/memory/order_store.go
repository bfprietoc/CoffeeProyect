package memory

import (
	"coffeeproyect/internal/domain"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// newUUID genera un UUID v4 con crypto/rand — el mismo formato que usa la
// base (postgres) para los IDs en producción, para que el modo in-memory
// se comporte como producción (URLs, regex de E2E, etc.).
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func nextOrderID() string {
	return newUUID()
}

func nextItemID() string {
	return newUUID()
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
	if order.PaymentStatus == "" {
		order.PaymentStatus = domain.PaymentStatusPending
	}
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

// UpdatePayment replaces the payment fields of one order.
// A nil field keeps the current value (except PaidAt, which is set to nil
// when the caller passes paid=false semantics via SetPaidAt(false, ...)).
func (s *OrderStore) GetByPaymentReference(reference string) (domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.orders {
		if o.PaymentReference != nil && *o.PaymentReference == reference {
			return o, nil
		}
	}
	return domain.Order{}, domain.ErrNotFound
}

func (s *OrderStore) UpdatePayment(id string, patch domain.PaymentPatch) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	if patch.Reference != nil {
		o.PaymentReference = patch.Reference
	}
	if patch.TransactionID != nil {
		o.PaymentTransactionID = patch.TransactionID
	}
	if patch.Provider != nil {
		o.PaymentProvider = patch.Provider
	}
	if patch.SetStatus {
		o.PaymentStatus = patch.Status
	}
	if patch.SetPaidAt {
		o.PaidAt = patch.PaidAt
	}
	o.UpdatedAt = time.Now().UTC()
	s.orders[id] = o
	return o, nil
}
