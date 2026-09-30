package store

import "coffeeproyect/internal/domain"

type WaitlistStore interface {
	Subscribe(entry domain.WaitlistEntry) (domain.WaitlistEntry, error)
	GetPending(coffeeID string) ([]domain.WaitlistEntry, error)
	MarkNotified(id string) error
}
