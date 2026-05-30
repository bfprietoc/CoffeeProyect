package postgres

import (
	"coffeeproyect/internal/domain"
	"database/sql"
	"errors"
	"fmt"
)

type CartStore struct {
	db *sql.DB
}

func NewCartStore(db *sql.DB) *CartStore {
	return &CartStore{db: db}
}

func (s *CartStore) GetItems(userID string) ([]domain.CartItem, error) {
	rows, err := s.db.Query(`
		SELECT ci.coffee_id, c.name, ci.quantity, ci.unit_price_cents, c.bag_size_grams
		FROM cart_items ci
		JOIN coffees c ON ci.coffee_id = c.id
		WHERE ci.cart_id = (SELECT id FROM carts WHERE user_id = $1)
		ORDER BY ci.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("get cart items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.CartItem, 0)
	for rows.Next() {
		var it domain.CartItem
		if err := rows.Scan(&it.CoffeeID, &it.CoffeeName, &it.Quantity,
			&it.UnitPriceCents, &it.BagSizeGrams); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *CartStore) AddItem(userID string, item domain.CartItem) error {
	cartID, err := s.getOrCreateCart(userID)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO cart_items (cart_id, coffee_id, quantity, unit_price_cents)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cart_id, coffee_id) DO UPDATE
		SET quantity         = cart_items.quantity + EXCLUDED.quantity,
		    unit_price_cents = EXCLUDED.unit_price_cents`,
		cartID, item.CoffeeID, item.Quantity, item.UnitPriceCents)
	return err
}

func (s *CartStore) SetQuantity(userID, coffeeID string, quantity int) error {
	res, err := s.db.Exec(`
		UPDATE cart_items
		SET quantity = $1
		WHERE cart_id = (SELECT id FROM carts WHERE user_id = $2)
		  AND coffee_id = $3`,
		quantity, userID, coffeeID)
	if err != nil {
		return fmt.Errorf("set quantity: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *CartStore) RemoveItem(userID, coffeeID string) error {
	res, err := s.db.Exec(`
		DELETE FROM cart_items
		WHERE cart_id = (SELECT id FROM carts WHERE user_id = $1)
		  AND coffee_id = $2`,
		userID, coffeeID)
	if err != nil {
		return fmt.Errorf("remove item: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *CartStore) Clear(userID string) error {
	_, err := s.db.Exec(`
		DELETE FROM cart_items
		WHERE cart_id = (SELECT id FROM carts WHERE user_id = $1)`, userID)
	return err
}

func (s *CartStore) getOrCreateCart(userID string) (string, error) {
	_, err := s.db.Exec(
		`INSERT INTO carts (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID)
	if err != nil {
		return "", fmt.Errorf("upsert cart: %w", err)
	}

	var cartID string
	err = s.db.QueryRow(`SELECT id FROM carts WHERE user_id = $1`, userID).Scan(&cartID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return cartID, err
}
