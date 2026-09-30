package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/store"
)

type ValidationItem struct {
	CoffeeID     string `json:"coffee_id"`
	CoffeeName   string `json:"coffee_name"`
	RequestedQty int    `json:"quantity"`
	StockBags    int    `json:"stock_bags"`
	Available    bool   `json:"available"`
}

type ValidationResult struct {
	Items       []ValidationItem `json:"items"`
	TotalCents  int              `json:"total_cents"`
	Currency    string           `json:"currency"`
	CanCheckout bool             `json:"can_checkout"`
}

type CheckoutService struct {
	cartStore   store.CartStore
	coffeeStore store.CoffeeStore
	orderStore  store.OrderStore
	userStore   store.UserStore
	publisher   event.Publisher // nil = no event publishing
}

func NewCheckoutService(
	cart store.CartStore,
	coffee store.CoffeeStore,
	orders store.OrderStore,
	users store.UserStore,
) *CheckoutService {
	return &CheckoutService{
		cartStore:   cart,
		coffeeStore: coffee,
		orderStore:  orders,
		userStore:   users,
	}
}

func (s *CheckoutService) WithPublisher(p event.Publisher) *CheckoutService {
	s.publisher = p
	return s
}

// Validate checks every cart item against current stock without writing anything.
func (s *CheckoutService) Validate(userID string) (ValidationResult, error) {
	items, err := s.cartStore.GetItems(userID)
	if err != nil {
		return ValidationResult{}, err
	}

	result := ValidationResult{
		Items:    make([]ValidationItem, 0, len(items)),
		Currency: "COP",
	}
	canCheckout := len(items) > 0

	for _, ci := range items {
		coffee, err := s.coffeeStore.GetByID(ci.CoffeeID)
		vi := ValidationItem{
			CoffeeID:     ci.CoffeeID,
			CoffeeName:   ci.CoffeeName,
			RequestedQty: ci.Quantity,
		}
		if err == nil {
			vi.StockBags = coffee.StockBags
			vi.Available = coffee.Available && coffee.StockBags >= ci.Quantity
			if vi.Available {
				result.TotalCents += ci.Quantity * coffee.PriceCents
			} else {
				canCheckout = false
			}
		} else {
			canCheckout = false
		}
		result.Items = append(result.Items, vi)
	}
	result.CanCheckout = canCheckout
	return result, nil
}

// PlaceOrder validates stock, creates the order atomically, and clears the cart.
func (s *CheckoutService) PlaceOrder(userID, addressID string) (domain.Order, error) {
	items, err := s.cartStore.GetItems(userID)
	if err != nil {
		return domain.Order{}, err
	}
	if len(items) == 0 {
		return domain.Order{}, domain.ErrCartEmpty
	}

	addr, err := s.findAddress(userID, addressID)
	if err != nil {
		return domain.Order{}, err
	}

	orderItems := make([]domain.OrderItem, 0, len(items))
	totalCents := 0
	for _, ci := range items {
		coffee, err := s.coffeeStore.GetByID(ci.CoffeeID)
		if err != nil {
			return domain.Order{}, err
		}
		if !coffee.Available || coffee.StockBags < ci.Quantity {
			return domain.Order{}, domain.ErrInsufficientStock
		}
		orderItems = append(orderItems, domain.OrderItem{
			CoffeeID:       ci.CoffeeID,
			CoffeeName:     coffee.Name,
			Quantity:       ci.Quantity,
			UnitPriceCents: coffee.PriceCents,
		})
		totalCents += ci.Quantity * coffee.PriceCents
	}

	order := domain.Order{
		UserID:          userID,
		Items:           orderItems,
		ShippingStreet:  addr.Street,
		ShippingCity:    addr.City,
		ShippingDept:    addr.Department,
		ShippingCountry: addr.Country,
		TotalCents:      totalCents,
		Currency:        "COP",
		Status:          domain.OrderStatusConfirmed,
	}

	created, err := s.orderStore.Create(order)
	if err != nil {
		return domain.Order{}, err
	}

	// best-effort cart clear
	_ = s.cartStore.Clear(userID)

	if s.publisher != nil {
		_ = s.publisher.Publish(event.TopicOrderCreated, event.OrderPayload{
			OrderID: created.ID,
			UserID:  userID,
		})
	}

	return created, nil
}

func (s *CheckoutService) findAddress(userID, addressID string) (domain.Address, error) {
	addresses, err := s.userStore.GetAddresses(userID)
	if err != nil {
		return domain.Address{}, err
	}
	for _, a := range addresses {
		if a.ID == addressID {
			return a, nil
		}
	}
	return domain.Address{}, domain.ErrNotFound
}
