package domain

import "time"

type StockMovement struct {
	ID          string    `json:"id"`
	CoffeeID    string    `json:"coffee_id"`
	Delta       int       `json:"delta"`        // positive = add, negative = subtract
	ResultStock int       `json:"result_stock"` // stock after this movement
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}
