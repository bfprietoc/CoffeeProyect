package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/notification"
	"coffeeproyect/internal/store"
	"fmt"
	"strings"
)

type NotificationService struct {
	email      notification.EmailSender
	orderStore store.OrderStore
	userStore  store.UserStore
}

func NewNotificationService(
	email notification.EmailSender,
	orders store.OrderStore,
	users store.UserStore,
) *NotificationService {
	return &NotificationService{email: email, orderStore: orders, userStore: users}
}

func (s *NotificationService) OnOrderCreated(orderID, userID string) error {
	if s.email == nil {
		return nil
	}
	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return fmt.Errorf("notification: get order: %w", err)
	}
	user, err := s.userStore.GetByID(userID)
	if err != nil {
		return fmt.Errorf("notification: get user: %w", err)
	}
	return s.email.Send(
		user.Email,
		fmt.Sprintf("Confirmamos tu pedido #%s", order.ID),
		orderConfirmationBody(order),
	)
}

func (s *NotificationService) OnOrderShipped(orderID, userID string) error {
	if s.email == nil {
		return nil
	}
	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return fmt.Errorf("notification: get order: %w", err)
	}
	user, err := s.userStore.GetByID(userID)
	if err != nil {
		return fmt.Errorf("notification: get user: %w", err)
	}
	tracking := "no disponible"
	if order.TrackingNumber != nil {
		tracking = *order.TrackingNumber
	}
	body := fmt.Sprintf(
		"Tu pedido #%s está en camino.\n\nNúmero de seguimiento: %s\n\nDirección de entrega:\n  %s\n  %s, %s, %s",
		order.ID, tracking,
		order.ShippingStreet, order.ShippingCity, order.ShippingDept, order.ShippingCountry,
	)
	return s.email.Send(user.Email, "Tu pedido está en camino", body)
}

func (s *NotificationService) OnOrderDelivered(orderID, userID string) error {
	if s.email == nil {
		return nil
	}
	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return fmt.Errorf("notification: get order: %w", err)
	}
	user, err := s.userStore.GetByID(userID)
	if err != nil {
		return fmt.Errorf("notification: get user: %w", err)
	}
	body := fmt.Sprintf(
		"¡Tu pedido #%s llegó!\n\nEsperamos que disfrutes tu café. Si tienes algún inconveniente, contáctanos.",
		order.ID,
	)
	return s.email.Send(user.Email, "Tu pedido llegó", body)
}

func (s *NotificationService) OnOrderCancelled(orderID, userID string) error {
	if s.email == nil {
		return nil
	}
	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return fmt.Errorf("notification: get order: %w", err)
	}
	user, err := s.userStore.GetByID(userID)
	if err != nil {
		return fmt.Errorf("notification: get user: %w", err)
	}
	body := fmt.Sprintf(
		"Tu pedido #%s ha sido cancelado.\n\nTotal: %s %d\n\nSi tienes preguntas, contáctanos.",
		order.ID, order.Currency, order.TotalCents/100,
	)
	return s.email.Send(user.Email, "Tu orden fue cancelada", body)
}

func (s *NotificationService) OnUserRegistered(email, name string) error {
	if s.email == nil {
		return nil
	}
	body := fmt.Sprintf(
		"Hola %s,\n\nBienvenido a CoffeeProyect. Explora nuestra selección de cafés especiales colombianos de origen.\n\nNos alegra tenerte aquí.",
		name,
	)
	return s.email.Send(email, "Bienvenido a CoffeeProyect", body)
}

func orderConfirmationBody(order domain.Order) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Pedido #%s\n\nÍtems:\n", order.ID)
	for _, item := range order.Items {
		fmt.Fprintf(&sb, "  - %s x%d: %s %d\n",
			item.CoffeeName, item.Quantity, order.Currency, item.SubtotalCents/100)
	}
	fmt.Fprintf(&sb, "\nTotal: %s %d\n", order.Currency, order.TotalCents/100)
	fmt.Fprintf(&sb, "\nDirección de envío:\n  %s\n  %s, %s, %s\n",
		order.ShippingStreet, order.ShippingCity, order.ShippingDept, order.ShippingCountry)
	return sb.String()
}
