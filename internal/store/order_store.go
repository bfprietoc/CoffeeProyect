package store

import "coffeeproyect/internal/domain"

type OrderStore interface {
	// Create atomically deducts stock for each item and inserts the order.
	// Returns ErrInsufficientStock if any item has insufficient stock.
	Create(order domain.Order) (domain.Order, error)

	GetByID(id string) (domain.Order, error)
	GetByIDAndUser(id, userID string) (domain.Order, error)
	ListByUser(userID string) ([]domain.Order, error)
	// ListAll is for admin — statusFilter empty means all statuses.
	ListAll(statusFilter string, limit, offset int) ([]domain.Order, error)

	// Cancel atomically restores stock and marks the order as cancelled.
	// Returns ErrNotFound if order doesn't exist or doesn't belong to userID.
	// Returns ErrOrderNotCancellable if the current status disallows it.
	Cancel(orderID, userID string) (domain.Order, error)

	// UpdateStatus is for admin use. trackingNumber may be nil.
	UpdateStatus(id string, status domain.OrderStatus, trackingNumber *string) (domain.Order, error)
}
