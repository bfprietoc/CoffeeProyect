package memory

import (
	"coffeeproyect/internal/domain"
	"sync"
)

type WaitlistStore struct {
	mu      sync.Mutex
	entries map[string]domain.WaitlistEntry // id → entry
}

func NewWaitlistStore() *WaitlistStore {
	return &WaitlistStore{entries: make(map[string]domain.WaitlistEntry)}
}

func (s *WaitlistStore) Subscribe(entry domain.WaitlistEntry) (domain.WaitlistEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// idempotent: skip if already subscribed
	for _, e := range s.entries {
		if e.CoffeeID == entry.CoffeeID && e.Email == entry.Email {
			return e, nil
		}
	}
	entry.ID = nextOrderID()
	s.entries[entry.ID] = entry
	return entry, nil
}

func (s *WaitlistStore) GetPending(coffeeID string) ([]domain.WaitlistEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]domain.WaitlistEntry, 0)
	for _, e := range s.entries {
		if e.CoffeeID == coffeeID && !e.Notified {
			result = append(result, e)
		}
	}
	return result, nil
}

func (s *WaitlistStore) MarkNotified(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Notified = true
	s.entries[id] = e
	return nil
}
