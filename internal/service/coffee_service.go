package service

import (
	"coffeeproyect/internal/cache"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
)

type CoffeeService struct {
	store store.CoffeeStore
	cache cache.CoffeeCache
}

func NewCoffeeService(s store.CoffeeStore) *CoffeeService {
	return &CoffeeService{store: s}
}

func (s *CoffeeService) WithCache(c cache.CoffeeCache) *CoffeeService {
	s.cache = c
	return s
}

// InvalidateProduct clears cached data for a product. Called when stock changes.
func (s *CoffeeService) InvalidateProduct(id string) {
	if s.cache != nil {
		s.cache.Invalidate(id)
	}
}

func (s *CoffeeService) GetByID(id string) (domain.Coffee, error) {
	if s.cache != nil {
		if coffee, ok := s.cache.GetByID(id); ok {
			return coffee, nil
		}
	}
	coffee, err := s.store.GetByID(id)
	if err != nil {
		return domain.Coffee{}, err
	}
	if s.cache != nil {
		s.cache.SetByID(id, coffee)
	}
	return coffee, nil
}

func (s *CoffeeService) Create(input domain.Coffee) (domain.Coffee, error) {
	if input.Name == "" || input.Process == "" || input.RoastLevel == "" || input.PriceCents <= 0 {
		return domain.Coffee{}, domain.ErrInvalidInput
	}
	if input.Currency == "" {
		input.Currency = "COP"
	}
	if input.TastingNotes == nil {
		input.TastingNotes = []string{}
	}
	created, err := s.store.Create(input)
	if err != nil {
		return domain.Coffee{}, err
	}
	if s.cache != nil {
		s.cache.InvalidateAll()
	}
	return created, nil
}

func (s *CoffeeService) Update(id string, input domain.Coffee) (domain.Coffee, error) {
	if input.Name == "" || input.Process == "" || input.RoastLevel == "" || input.PriceCents <= 0 {
		return domain.Coffee{}, domain.ErrInvalidInput
	}
	existing, err := s.GetByID(id)
	if err != nil {
		return domain.Coffee{}, err
	}
	input.ID = id
	input.StockBags = existing.StockBags
	input.Available = existing.StockBags > 0
	input.Producer = existing.Producer
	input.Farm = existing.Farm
	if input.Currency == "" {
		input.Currency = existing.Currency
	}
	if input.TastingNotes == nil {
		input.TastingNotes = []string{}
	}
	updated, err := s.store.Update(input)
	if err != nil {
		return domain.Coffee{}, err
	}
	s.InvalidateProduct(id)
	return updated, nil
}

func (s *CoffeeService) Delete(id string) error {
	if _, err := s.store.GetByID(id); err != nil {
		return err
	}
	if err := s.store.Delete(id); err != nil {
		return err
	}
	s.InvalidateProduct(id)
	return nil
}

func (s *CoffeeService) List(filters store.CoffeeFilters) ([]domain.Coffee, error) {
	if s.cache != nil {
		key := cache.ListKey(filters)
		if coffees, ok := s.cache.GetList(key); ok {
			return coffees, nil
		}
	}
	coffees, err := s.store.List(filters)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.SetList(cache.ListKey(filters), coffees)
	}
	return coffees, nil
}
