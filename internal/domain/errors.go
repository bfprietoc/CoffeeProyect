package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrInsufficientStock   = errors.New("insufficient stock")
	ErrInvalidInput        = errors.New("invalid input")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrCartEmpty           = errors.New("cart is empty")
	ErrOrderNotCancellable = errors.New("order cannot be cancelled in its current status")
	ErrCoffeeHasOrders     = errors.New("coffee has orders and cannot be deleted")
)

// ErrPaymentAlreadyPaid indicates the order was already paid (duplicate
// payment attempt). Mapped to 409 Conflict.
var ErrPaymentAlreadyPaid = errors.New("order already paid")

// ErrPaymentUnavailable indicates the order cannot be paid in its current
// state (e.g. cancelled). Mapped to 422 Unprocessable Entity.
var ErrPaymentUnavailable = errors.New("order cannot be paid in its current state")

// ErrPaymentDisabled indicates payments are not configured (missing env).
// Mapped to 503 Service Unavailable.
var ErrPaymentDisabled = errors.New("payments are disabled")

// ErrPaymentRequired indicates the order must be paid before it can be
// moved into preparation. Mapped to 422 Unprocessable Entity.
var ErrPaymentRequired = errors.New("order must be paid before processing")
