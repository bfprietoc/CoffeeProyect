package store

import "coffeeproyect/internal/domain"

// CartStore es la interfaz unificada para carritos de invitados (Redis)
// y de usuarios registrados (PostgreSQL). El cartID es sessionID para
// invitados y userID para usuarios.
type CartStore interface {
	GetItems(cartID string) ([]domain.CartItem, error)
	AddItem(cartID string, item domain.CartItem) error
	SetQuantity(cartID, coffeeID string, quantity int) error
	RemoveItem(cartID, coffeeID string) error
	Clear(cartID string) error
}
