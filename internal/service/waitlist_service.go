package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/notification"
	"coffeeproyect/internal/store"
	"fmt"
)

type WaitlistService struct {
	waitlistStore store.WaitlistStore
	coffeeStore   store.CoffeeStore
	email         notification.EmailSender // nil = no email
	publisher     event.Publisher          // nil = no events
}

func NewWaitlistService(
	wl store.WaitlistStore,
	coffee store.CoffeeStore,
	email notification.EmailSender,
	publisher event.Publisher,
) *WaitlistService {
	return &WaitlistService{
		waitlistStore: wl,
		coffeeStore:   coffee,
		email:         email,
		publisher:     publisher,
	}
}

// Subscribe adds the email to the waitlist for a coffee that is currently out of stock.
// Returns ErrInvalidInput if the coffee is available (no need to subscribe).
func (s *WaitlistService) Subscribe(coffeeID, email string, userID *string) (domain.WaitlistEntry, error) {
	coffee, err := s.coffeeStore.GetByID(coffeeID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if coffee.Available {
		return domain.WaitlistEntry{}, fmt.Errorf("%w: coffee is currently in stock", domain.ErrInvalidInput)
	}

	entry, err := s.waitlistStore.Subscribe(domain.WaitlistEntry{
		CoffeeID: coffeeID,
		UserID:   userID,
		Email:    email,
	})
	if err != nil {
		return domain.WaitlistEntry{}, err
	}

	if s.email != nil {
		_ = s.email.Send(email, "Te avisaremos cuando llegue",
			fmt.Sprintf("Te has suscrito a la lista de espera para %s. Te avisaremos cuando esté disponible.", coffee.Name))
	}
	if s.publisher != nil {
		_ = s.publisher.Publish(event.TopicWaitlistSubscribed, event.WaitlistPayload{
			CoffeeID: coffeeID, Email: email,
		})
	}

	return entry, nil
}

// NotifyAll sends emails to all pending waitlist subscribers for a coffee.
// Designed to be called when stock.restored fires.
func (s *WaitlistService) NotifyAll(coffeeID string) error {
	coffee, err := s.coffeeStore.GetByID(coffeeID)
	if err != nil {
		return err
	}

	entries, err := s.waitlistStore.GetPending(coffeeID)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if s.email != nil {
			_ = s.email.Send(e.Email,
				fmt.Sprintf("¡%s ya está disponible!", coffee.Name),
				fmt.Sprintf("¡Buenas noticias! %s volvió a estar disponible. Cómpralo antes de que se agote.", coffee.Name),
			)
		}
		_ = s.waitlistStore.MarkNotified(e.ID)
	}
	return nil
}
