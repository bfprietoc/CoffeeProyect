package store

import "coffeeproyect/internal/domain"

type UserStore interface {
	Create(user domain.User) (domain.User, error)
	GetByEmail(email string) (domain.User, error)
	GetByID(id string) (domain.User, error)
	Update(user domain.User) (domain.User, error)
	AddAddress(address domain.Address) (domain.Address, error)
	GetAddresses(userID string) ([]domain.Address, error)
	DeleteAddress(id, userID string) error
}
