package event

// Topic constants for all domain events.
const (
	TopicOrderCreated   = "order.created"
	TopicOrderCancelled = "order.cancelled"
	TopicOrderShipped   = "order.shipped"
	TopicOrderDelivered = "order.delivered"

	TopicStockDecremented = "stock.decremented"
	TopicStockReplenished = "stock.replenished"
	TopicStockDepleted    = "stock.depleted"
	TopicStockRestored    = "stock.restored" // stock went from 0 to > 0
	TopicStockLow         = "stock.low"

	TopicUserRegistered     = "user.registered"
	TopicWaitlistSubscribed = "waitlist.subscribed"
)

type OrderPayload struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
}

type StockPayload struct {
	CoffeeID    string `json:"coffee_id"`
	Delta       int    `json:"delta"`
	OldStock    int    `json:"old_stock"`
	ResultStock int    `json:"result_stock"`
	Note        string `json:"note,omitempty"`
}

type UserPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type WaitlistPayload struct {
	CoffeeID string `json:"coffee_id"`
	Email    string `json:"email"`
}
