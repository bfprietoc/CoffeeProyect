package memory

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

var coffeeSeq int64

type CoffeeStore struct {
	mu      sync.RWMutex
	coffees map[string]domain.Coffee
}

func NewCoffeeStore() *CoffeeStore {
	cs := &CoffeeStore{coffees: make(map[string]domain.Coffee)}
	for _, c := range seedCoffees() {
		cs.coffees[c.ID] = c
	}
	return cs
}

func (s *CoffeeStore) GetByID(id string) (domain.Coffee, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.coffees[id]
	if !ok {
		return domain.Coffee{}, domain.ErrNotFound
	}
	return c, nil
}

func (s *CoffeeStore) List(filters store.CoffeeFilters) ([]domain.Coffee, error) {
	s.mu.RLock()
	all := make([]domain.Coffee, 0, len(s.coffees))
	for _, c := range s.coffees {
		all = append(all, c)
	}
	s.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	result := make([]domain.Coffee, 0)
	for _, c := range all {
		if filters.Available != nil && c.Available != *filters.Available {
			continue
		}
		if filters.RoastLevel != "" && c.RoastLevel != filters.RoastLevel {
			continue
		}
		if filters.Country != "" && c.Farm.Country != filters.Country {
			continue
		}
		if filters.Process != "" && c.Process != filters.Process {
			continue
		}
		result = append(result, c)
	}

	limit := filters.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	page := filters.Page
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= len(result) {
		return []domain.Coffee{}, nil
	}
	end := start + limit
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}

func (s *CoffeeStore) Create(coffee domain.Coffee) (domain.Coffee, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if coffee.ID == "" {
		coffee.ID = fmt.Sprintf("mem-coffee-%d", atomic.AddInt64(&coffeeSeq, 1))
	}
	if coffee.Currency == "" {
		coffee.Currency = "COP"
	}
	if coffee.TastingNotes == nil {
		coffee.TastingNotes = []string{}
	}
	coffee.Available = coffee.StockBags > 0
	if coffee.Producer.ID == "" {
		coffee.Producer.ID = fmt.Sprintf("mem-producer-%s", coffee.ID)
	}
	if coffee.Farm.ID == "" {
		coffee.Farm.ID = fmt.Sprintf("mem-farm-%s", coffee.ID)
	}
	s.coffees[coffee.ID] = coffee
	return coffee, nil
}

func (s *CoffeeStore) Update(coffee domain.Coffee) (domain.Coffee, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.coffees[coffee.ID]; !ok {
		return domain.Coffee{}, domain.ErrNotFound
	}
	coffee.Available = coffee.StockBags > 0
	s.coffees[coffee.ID] = coffee
	return coffee, nil
}

func (s *CoffeeStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.coffees[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.coffees, id)
	return nil
}

func seedCoffees() []domain.Coffee {
	trueVal := true

	return []domain.Coffee{
		{
			ID:           "c1a2b3c4-0001-0001-0001-000000000001",
			Name:         "El Paraíso 92",
			Process:      "anaerobic",
			RoastLevel:   "light",
			TastingNotes: []string{"maracuyá", "uva", "chocolate negro"},
			Description:  "Café de proceso anaeróbico de la finca El Paraíso en el Cauca. Fermentación controlada de 48h que potencia sus notas afrutadas y una acidez vibrante.",
			Producer:     domain.Producer{ID: "p1", Name: "Diego Samuel Bermúdez"},
			Farm:         domain.Farm{ID: "f1", Name: "El Paraíso", Country: "Colombia", Region: "Cauca"},
			BagSizeGrams: 250,
			PriceCents:   8500000,
			Currency:     "COP",
			StockBags:    12,
			Available:    trueVal,
		},
		{
			ID:           "c1a2b3c4-0002-0002-0002-000000000002",
			Name:         "Finca La Esperanza Wush Wush",
			Process:      "washed",
			RoastLevel:   "light",
			TastingNotes: []string{"jazmín", "durazno", "té negro"},
			Description:  "Variedad Wush Wush de origen etíope cultivada en el Valle del Cauca. Proceso lavado que resalta su delicada acidez floral y cuerpo sedoso.",
			Producer:     domain.Producer{ID: "p2", Name: "Camilo Merizalde"},
			Farm:         domain.Farm{ID: "f2", Name: "La Esperanza", Country: "Colombia", Region: "Valle del Cauca"},
			BagSizeGrams: 250,
			PriceCents:   9200000,
			Currency:     "COP",
			StockBags:    8,
			Available:    trueVal,
		},
		{
			ID:           "c1a2b3c4-0003-0003-0003-000000000003",
			Name:         "Huila Natural Caturra",
			Process:      "natural",
			RoastLevel:   "medium",
			TastingNotes: []string{"panela", "cereza", "almendra"},
			Description:  "Caturra de proceso natural del Huila. Secado en camas africanas durante 25 días. Dulzura prominente con cuerpo redondo y un final largo a frutos rojos.",
			Producer:     domain.Producer{ID: "p3", Name: "Familia Muñoz"},
			Farm:         domain.Farm{ID: "f3", Name: "Finca El Oasis", Country: "Colombia", Region: "Huila"},
			BagSizeGrams: 340,
			PriceCents:   5500000,
			Currency:     "COP",
			StockBags:    25,
			Available:    trueVal,
		},
		{
			ID:           "c1a2b3c4-0004-0004-0004-000000000004",
			Name:         "Nariño Honey Gesha",
			Process:      "honey",
			RoastLevel:   "light",
			TastingNotes: []string{"bergamota", "mango", "miel de caña"},
			Description:  "Gesha en proceso honey de los altiplanos de Nariño a 2.100 msnm. Mucílago retenido al 50% para una dulzura equilibrada con la acidez brillante característica del terroir nariñense.",
			Producer:     domain.Producer{ID: "p4", Name: "Asociación Cafeteros de Nariño"},
			Farm:         domain.Farm{ID: "f4", Name: "Alto de la Cruz", Country: "Colombia", Region: "Nariño"},
			BagSizeGrams: 250,
			PriceCents:   11000000,
			Currency:     "COP",
			StockBags:    6,
			Available:    trueVal,
		},
		{
			ID:           "c1a2b3c4-0005-0005-0005-000000000005",
			Name:         "Antioquia Dark Roast Blend",
			Process:      "washed",
			RoastLevel:   "dark",
			TastingNotes: []string{"chocolate amargo", "caramelo", "nuez"},
			Description:  "Blend de fincas del suroeste antioqueño con tueste oscuro. Diseñado para espresso y métodos de alta presión. Cuerpo denso y dulzura de caramelo sin acidez invasiva.",
			Producer:     domain.Producer{ID: "p5", Name: "Cooperativa Andes"},
			Farm:         domain.Farm{ID: "f5", Name: "Varias fincas", Country: "Colombia", Region: "Antioquia"},
			BagSizeGrams: 500,
			PriceCents:   4800000,
			Currency:     "COP",
			StockBags:    0,
			Available:    false,
		},
	}
}
