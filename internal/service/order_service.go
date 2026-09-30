package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/store"
)

type OrderService struct {
	orderStore store.OrderStore
	publisher  event.Publisher
}

func NewOrderService(orders store.OrderStore) *OrderService {
	return &OrderService{orderStore: orders}
}

func (s *OrderService) WithPublisher(p event.Publisher) *OrderService {
	s.publisher = p
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

	if s.publisher != nil {
		_ = s.publisher.Publish(event.TopicOrderCancelled, event.OrderPayload{
			OrderID: order.ID,
			UserID:  order.UserID,
		})
	}

	return order, nil
}
