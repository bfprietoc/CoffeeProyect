package postgres

import (
	"coffeeproyect/internal/domain"
	"database/sql"
	"errors"
	"fmt"
)

type OrderStore struct {
	db *sql.DB
}

func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// Create runs an atomic transaction: stock deduction + order insert + order_items insert.
func (s *OrderStore) Create(order domain.Order) (domain.Order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Order{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, item := range order.Items {
		var stock int
		err := tx.QueryRow(
			`SELECT stock_bags FROM coffees WHERE id = $1 FOR UPDATE`,
			item.CoffeeID,
		).Scan(&stock)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Order{}, domain.ErrNotFound
		}
		if err != nil {
			return domain.Order{}, fmt.Errorf("lock coffee %s: %w", item.CoffeeID, err)
		}
		if stock < item.Quantity {
			return domain.Order{}, fmt.Errorf("%w: %s has %d bags, need %d",
				domain.ErrInsufficientStock, item.CoffeeName, stock, item.Quantity)
		}
		if _, err := tx.Exec(
			`UPDATE coffees SET stock_bags = stock_bags - $1 WHERE id = $2`,
			item.Quantity, item.CoffeeID,
		); err != nil {
			return domain.Order{}, fmt.Errorf("deduct stock %s: %w", item.CoffeeID, err)
		}
	}

	err = tx.QueryRow(`
		INSERT INTO orders
		    (user_id, shipping_street, shipping_city, shipping_department, shipping_country,
		     total_cents, currency, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'confirmed')
		RETURNING id, created_at, updated_at`,
		order.UserID, order.ShippingStreet, order.ShippingCity,
		order.ShippingDept, order.ShippingCountry,
		order.TotalCents, order.Currency,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return domain.Order{}, fmt.Errorf("insert order: %w", err)
	}
	order.Status = domain.OrderStatusConfirmed

	for i, item := range order.Items {
		err := tx.QueryRow(`
			INSERT INTO order_items (order_id, coffee_id, coffee_name, quantity, unit_price_cents)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			order.ID, item.CoffeeID, item.CoffeeName, item.Quantity, item.UnitPriceCents,
		).Scan(&order.Items[i].ID)
		if err != nil {
			return domain.Order{}, fmt.Errorf("insert order_item: %w", err)
		}
		order.Items[i].SubtotalCents = item.Quantity * item.UnitPriceCents
	}

	return order, tx.Commit()
}

func (s *OrderStore) GetByID(id string) (domain.Order, error) {
	return s.scanOrder(`WHERE o.id = $1`, id)
}

func (s *OrderStore) GetByIDAndUser(id, userID string) (domain.Order, error) {
	o, err := s.scanOrder(`WHERE o.id = $1`, id)
	if err != nil {
		return domain.Order{}, err
	}
	if o.UserID != userID {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, nil
}

func (s *OrderStore) ListByUser(userID string) ([]domain.Order, error) {
	return s.scanOrders(`WHERE o.user_id = $1 ORDER BY o.created_at DESC`, userID)
}

func (s *OrderStore) ListAll(statusFilter string, limit, offset int) ([]domain.Order, error) {
	if limit <= 0 {
		limit = 20
	}
	if statusFilter != "" {
		return s.scanOrders(
			`WHERE o.status = $1 ORDER BY o.created_at DESC LIMIT $2 OFFSET $3`,
			statusFilter, limit, offset)
	}
	return s.scanOrders(`ORDER BY o.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
}

// Cancel atomically restores stock and marks the order cancelled.
func (s *OrderStore) Cancel(orderID, userID string) (domain.Order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Order{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRow(
		`SELECT status FROM orders WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		orderID, userID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("lock order: %w", err)
	}
	if !domain.OrderStatus(status).IsCancellable() {
		return domain.Order{}, domain.ErrOrderNotCancellable
	}

	// Collect items to restore before running any further statements on the tx.
	// lib/pq does not support interleaving queries with open result sets.
	type stockItem struct {
		coffeeID string
		qty      int
	}
	rows, err := tx.Query(
		`SELECT coffee_id, quantity FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return domain.Order{}, fmt.Errorf("list items: %w", err)
	}
	var toRestore []stockItem
	for rows.Next() {
		var it stockItem
		if err := rows.Scan(&it.coffeeID, &it.qty); err != nil {
			rows.Close()
			return domain.Order{}, err
		}
		toRestore = append(toRestore, it)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Order{}, err
	}
	rows.Close()

	for _, it := range toRestore {
		if _, err := tx.Exec(
			`UPDATE coffees SET stock_bags = stock_bags + $1 WHERE id = $2`, it.qty, it.coffeeID,
		); err != nil {
			return domain.Order{}, fmt.Errorf("restore stock: %w", err)
		}
	}

	var o domain.Order
	err = tx.QueryRow(
		`UPDATE orders SET status = 'cancelled' WHERE id = $1
		 RETURNING id, user_id, status, total_cents, currency, created_at, updated_at`,
		orderID,
	).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return domain.Order{}, fmt.Errorf("cancel order: %w", err)
	}
	return o, tx.Commit()
}

func (s *OrderStore) UpdateStatus(id string, status domain.OrderStatus, trackingNumber *string) (domain.Order, error) {
	var o domain.Order
	err := s.db.QueryRow(`
		UPDATE orders SET status = $1, tracking_number = $2
		WHERE id = $3
		RETURNING id, user_id, status, tracking_number, total_cents, currency, created_at, updated_at`,
		string(status), trackingNumber, id,
	).Scan(&o.ID, &o.UserID, &o.Status, &o.TrackingNumber, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("update status: %w", err)
	}
	return o, nil
}

// scanOrder fetches a single order with its items.
func (s *OrderStore) scanOrder(whereClause string, args ...any) (domain.Order, error) {
	orders, err := s.scanOrders(whereClause, args...)
	if err != nil {
		return domain.Order{}, err
	}
	if len(orders) == 0 {
		return domain.Order{}, domain.ErrNotFound
	}
	return orders[0], nil
}

func (s *OrderStore) scanOrders(whereClause string, args ...any) ([]domain.Order, error) {
	query := fmt.Sprintf(`
		SELECT o.id, o.user_id,
		       o.shipping_street, o.shipping_city, o.shipping_department, o.shipping_country,
		       o.total_cents, o.currency, o.status, o.tracking_number,
		       o.created_at, o.updated_at
		FROM orders o %s`, whereClause)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(
			&o.ID, &o.UserID,
			&o.ShippingStreet, &o.ShippingCity, &o.ShippingDept, &o.ShippingCountry,
			&o.TotalCents, &o.Currency, &o.Status, &o.TrackingNumber,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, o := range orders {
		items, err := s.loadItems(o.ID)
		if err != nil {
			return nil, err
		}
		orders[i].Items = items
	}
	return orders, nil
}

func (s *OrderStore) loadItems(orderID string) ([]domain.OrderItem, error) {
	rows, err := s.db.Query(`
		SELECT id, coffee_id, coffee_name, quantity, unit_price_cents
		FROM order_items WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(&it.ID, &it.CoffeeID, &it.CoffeeName,
			&it.Quantity, &it.UnitPriceCents); err != nil {
			return nil, err
		}
		it.SubtotalCents = it.Quantity * it.UnitPriceCents
		items = append(items, it)
	}
	return items, rows.Err()
}
