package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/store"
)

type OrderService struct {
	orderStore store.OrderStore
	publisher  event.Publisher
	coffeeSvc  *CoffeeService // nil = no cache invalidation
}

func NewOrderService(orders store.OrderStore) *OrderService {
	return &OrderService{orderStore: orders}
}

func (s *OrderService) WithPublisher(p event.Publisher) *OrderService {
	s.publisher = p
	return s
}

// WithCoffeeService enables catalog cache invalidation when stock changes.
func (s *OrderService) WithCoffeeService(cs *CoffeeService) *OrderService {
	s.coffeeSvc = cs
	return s
}

func (s *OrderService) GetByID(id, userID string) (domain.Order, error) {
	return s.orderStore.GetByIDAndUser(id, userID)
}

func (s *OrderService) ListByUser(userID string) ([]domain.Order, error) {
	return s.orderStore.ListByUser(userID)
}

func (s *OrderService) ListAll(statusFilter string, limit, offset int) ([]domain.Order, error) {
	return s.orderStore.ListAll(statusFilter, limit, offset)
}

func (s *OrderService) UpdateStatus(id string, status domain.OrderStatus, trackingNumber *string) (domain.Order, error) {
	// Business rule: an order can only enter the queue for preparation once
	// it has been paid. (Orders created before payment tracking are marked
	// paid by migration, so existing flows keep working.)
	if status == domain.OrderStatusProcessing {
		current, err := s.orderStore.GetByID(id)
		if err != nil {
			return domain.Order{}, err
		}
		if current.PaymentStatus != domain.PaymentStatusPaid {
			return domain.Order{}, domain.ErrPaymentRequired
		}
	}
	order, err := s.orderStore.UpdateStatus(id, status, trackingNumber)
	if err != nil {
		return domain.Order{}, err
	}
	if s.publisher != nil {
		switch status {
		case domain.OrderStatusShipped:
			_ = s.publisher.Publish(event.TopicOrderShipped, event.OrderPayload{
				OrderID: order.ID, UserID: order.UserID,
			})
		case domain.OrderStatusDelivered:
			_ = s.publisher.Publish(event.TopicOrderDelivered, event.OrderPayload{
				OrderID: order.ID, UserID: order.UserID,
			})
		}
	}
	return order, nil
}

func (s *OrderService) Cancel(orderID, userID string) (domain.Order, error) {
	order, err := s.orderStore.Cancel(orderID, userID)
	if err != nil {
		return domain.Order{}, err
	}

	// invalidate catalog cache: stock was restored for the order's coffees
	if s.coffeeSvc != nil {
		for _, it := range order.Items {
			s.coffeeSvc.InvalidateProduct(it.CoffeeID)
		}
	}

	if s.publisher != nil {
		_ = s.publisher.Publish(event.TopicOrderCancelled, event.OrderPayload{
			OrderID: order.ID,
			UserID:  order.UserID,
		})
	}

	return order, nil
}
