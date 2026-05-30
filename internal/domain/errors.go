package domain

import "errors"

var (
	ErrNotFound              = errors.New("not found")
	ErrInsufficientStock     = errors.New("insufficient stock")
	ErrInvalidInput          = errors.New("invalid input")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrCartEmpty             = errors.New("cart is empty")
	ErrOrderNotCancellable   = errors.New("order cannot be cancelled in its current status")
)
