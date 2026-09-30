package domain

import "time"

type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Address struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Label      string `json:"label"`
	Street     string `json:"street"`
	City       string `json:"city"`
	Department string `json:"department"`
	Country    string `json:"country"`
	IsDefault  bool   `json:"is_default"`
}
