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

func (s OrderStatus) IsCancellable() bool {
	switch s {
	case OrderStatusConfirmed, OrderStatusProcessing:
		return true
	}
	return false
}

type Order struct {
	ID              string      `json:"id"`
	UserID          string      `json:"user_id"`
	Items           []OrderItem `json:"items"`
	ShippingStreet  string      `json:"shipping_street"`
	ShippingCity    string      `json:"shipping_city"`
	ShippingDept    string      `json:"shipping_department"`
	ShippingCountry string      `json:"shipping_country"`
	TotalCents      int         `json:"total_cents"`
	Currency        string      `json:"currency"`
	Status          OrderStatus `json:"status"`
	TrackingNumber  *string     `json:"tracking_number,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID             string `json:"id"`
	CoffeeID       string `json:"coffee_id"`
	CoffeeName     string `json:"coffee_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	SubtotalCents  int    `json:"subtotal_cents"`
}
