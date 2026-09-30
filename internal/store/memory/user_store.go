package memory

import (
	"coffeeproyect/internal/domain"
	"sync"
)

type UserStore struct {
	mu        sync.RWMutex
	users     map[string]domain.User
	byEmail   map[string]string
	addresses map[string][]domain.Address
}

func NewUserStore() *UserStore {
	return &UserStore{
		users:     make(map[string]domain.User),
		byEmail:   make(map[string]string),
		addresses: make(map[string][]domain.Address),
	}
}

func (s *UserStore) Create(user domain.User) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[user.Email]; exists {
		return domain.User{}, domain.ErrEmailAlreadyExists
	}

	user.ID = newID()
	s.users[user.ID] = user
	s.byEmail[user.Email] = user.ID
	return user, nil
}

func (s *UserStore) GetByEmail(email string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byEmail[email]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return s.users[id], nil
}

func (s *UserStore) GetByID(id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (s *UserStore) Update(user domain.User) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[user.ID]; !ok {
		return domain.User{}, domain.ErrNotFound
	}
	s.users[user.ID] = user
	return user, nil
}

func (s *UserStore) AddAddress(address domain.Address) (domain.Address, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	address.ID = newID()

	if address.IsDefault {
		for i := range s.addresses[address.UserID] {
			s.addresses[address.UserID][i].IsDefault = false
		}
	}

	s.addresses[address.UserID] = append(s.addresses[address.UserID], address)
	return address, nil
}

func (s *UserStore) GetAddresses(userID string) ([]domain.Address, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := s.addresses[userID]
	if addrs == nil {
		return []domain.Address{}, nil
	}
	return addrs, nil
}

func (s *UserStore) DeleteAddress(id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	addrs := s.addresses[userID]
	for i, a := range addrs {
		if a.ID == id {
			s.addresses[userID] = append(addrs[:i], addrs[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func newID() string {
	// simple sequential-ish ID for testing — real IDs come from postgres
	return "mem-" + randomHex(8)
}

func randomHex(n int) string {
	const chars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[i%len(chars)]
	}
	return string(b)
}
