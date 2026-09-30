package store

import "coffeeproyect/internal/domain"

type CoffeeFilters struct {
	Available  *bool
	RoastLevel string
	Country    string
	Process    string
	Page       int
	Limit      int
}

type CoffeeStore interface {
	List(filters CoffeeFilters) ([]domain.Coffee, error)
	GetByID(id string) (domain.Coffee, error)
	Create(coffee domain.Coffee) (domain.Coffee, error)
	Update(coffee domain.Coffee) (domain.Coffee, error)
	Delete(id string) error
}
