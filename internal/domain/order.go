package domain

import "time"

type OrderStatus string

const (
	OrderStatusConfirmed  OrderStatus = "confirmed"
	OrderStatusProcessing OrderStatus = "processing"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusPaid    PaymentStatus = "paid"
	PaymentStatusFailed  PaymentStatus = "failed"
)

// IsTerminal reports whether the payment reached a final outcome.
func (s PaymentStatus) IsTerminal() bool {
	return s == PaymentStatusPaid || s == PaymentStatusFailed
}

// IsRetryable reports whether the payment may be attempted again.
func (s PaymentStatus) IsRetryable() bool {
	return s == PaymentStatusPending || s == PaymentStatusFailed
}

func (s OrderStatus) IsCancellable() bool {
	switch s {
	case OrderStatusConfirmed, OrderStatusProcessing:
		return true
	}
	return false
}

type Order struct {
	ID                   string        `json:"id"`
	UserID               string        `json:"user_id"`
	Items                []OrderItem   `json:"items"`
	ShippingStreet       string        `json:"shipping_street"`
	ShippingCity         string        `json:"shipping_city"`
	ShippingDept         string        `json:"shipping_department"`
	ShippingCountry      string        `json:"shipping_country"`
	TotalCents           int           `json:"total_cents"`
	Currency             string        `json:"currency"`
	Status               OrderStatus   `json:"status"`
	TrackingNumber       *string       `json:"tracking_number,omitempty"`
	PaymentStatus        PaymentStatus `json:"payment_status"`
	PaymentProvider      *string       `json:"payment_provider,omitempty"`
	PaymentReference     *string       `json:"payment_reference,omitempty"`
	PaymentTransactionID *string       `json:"payment_transaction_id,omitempty"`
	PaidAt               *time.Time    `json:"paid_at,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

type OrderItem struct {
	ID             string `json:"id"`
	CoffeeID       string `json:"coffee_id"`
	CoffeeName     string `json:"coffee_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	SubtotalCents  int    `json:"subtotal_cents"`
}

// PaymentPatch carries optional updates for an order's payment lifecycle.
// Nil pointers / false flags mean "keep current value".
type PaymentPatch struct {
	Reference     *string
	TransactionID *string
	Provider      *string
	Status        PaymentStatus
	SetStatus     bool
	PaidAt        *time.Time
	SetPaidAt     bool // explicit clear via PaidAt=nil
}
