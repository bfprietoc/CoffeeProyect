package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"errors"
	"fmt"
)

type CartService struct {
	guestStore  store.CartStore
	userStore   store.CartStore
	coffeeStore store.CoffeeStore
}

func NewCartService(guest, user store.CartStore, coffees store.CoffeeStore) *CartService {
	return &CartService{guestStore: guest, userStore: user, coffeeStore: coffees}
}

func (s *CartService) GetCart(id domain.CartIdentity) (domain.Cart, error) {
	items, err := s.storeFor(id).GetItems(id.ID)
	if err != nil {
		return domain.Cart{}, err
	}
	return buildCart(id.ID, items), nil
}

func (s *CartService) AddItem(id domain.CartIdentity, coffeeID string, quantity int) (domain.Cart, error) {
	if quantity <= 0 {
		return domain.Cart{}, domain.ErrInvalidInput
	}

	coffee, err := s.coffeeStore.GetByID(coffeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Cart{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Cart{}, fmt.Errorf("get coffee: %w", err)
	}
	if !coffee.Available {
		return domain.Cart{}, fmt.Errorf("coffee %s is not available: %w", coffeeID, domain.ErrInvalidInput)
	}

	item := domain.CartItem{
		CoffeeID:       coffee.ID,
		CoffeeName:     coffee.Name,
		Quantity:       quantity,
		UnitPriceCents: coffee.PriceCents,
		BagSizeGrams:   coffee.BagSizeGrams,
	}

	if err := s.storeFor(id).AddItem(id.ID, item); err != nil {
		return domain.Cart{}, err
	}

	return s.GetCart(id)
}

func (s *CartService) SetQuantity(id domain.CartIdentity, coffeeID string, quantity int) (domain.Cart, error) {
	if quantity <= 0 {
		return domain.Cart{}, domain.ErrInvalidInput
	}

	if err := s.storeFor(id).SetQuantity(id.ID, coffeeID, quantity); err != nil {
		return domain.Cart{}, err
	}

	return s.GetCart(id)
}

func (s *CartService) RemoveItem(id domain.CartIdentity, coffeeID string) (domain.Cart, error) {
	if err := s.storeFor(id).RemoveItem(id.ID, coffeeID); err != nil {
		return domain.Cart{}, err
	}
	return s.GetCart(id)
}

func (s *CartService) Clear(id domain.CartIdentity) error {
	return s.storeFor(id).Clear(id.ID)
}

// MergeGuestCart fusiona el carrito de invitado al carrito del usuario al hacer login.
// Ante ítems duplicados, toma la cantidad mayor. Luego elimina el carrito de invitado.
func (s *CartService) MergeGuestCart(sessionID, userID string) error {
	guestItems, err := s.guestStore.GetItems(sessionID)
	if err != nil || len(guestItems) == 0 {
		return err
	}

	userItems, err := s.userStore.GetItems(userID)
	if err != nil {
		return err
	}

	userQty := make(map[string]int, len(userItems))
	for _, it := range userItems {
		userQty[it.CoffeeID] = it.Quantity
	}

	for _, guestItem := range guestItems {
		if existingQty, exists := userQty[guestItem.CoffeeID]; exists {
			if guestItem.Quantity > existingQty {
				_ = s.userStore.SetQuantity(userID, guestItem.CoffeeID, guestItem.Quantity)
			}
		} else {
			_ = s.userStore.AddItem(userID, guestItem)
		}
	}

	return s.guestStore.Clear(sessionID)
}

func (s *CartService) storeFor(id domain.CartIdentity) store.CartStore {
	if id.IsGuest {
		return s.guestStore
	}
	return s.userStore
}

func buildCart(id string, items []domain.CartItem) domain.Cart {
	total := 0
	currency := "COP"
	enriched := make([]domain.CartItem, len(items))
	for i, it := range items {
		it.SubtotalCents = it.Quantity * it.UnitPriceCents
		total += it.SubtotalCents
		enriched[i] = it
	}
	return domain.Cart{
		ID:         id,
		Items:      enriched,
		TotalCents: total,
		Currency:   currency,
	}
}
