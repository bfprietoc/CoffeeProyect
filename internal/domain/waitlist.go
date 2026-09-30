package domain

import "time"

type WaitlistEntry struct {
	ID        string    `json:"id"`
	CoffeeID  string    `json:"coffee_id"`
	UserID    *string   `json:"user_id,omitempty"` // nil if guest
	Email     string    `json:"email"`
	Notified  bool      `json:"notified"`
	CreatedAt time.Time `json:"created_at"`
}
