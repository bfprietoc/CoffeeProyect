package redis

import (
	"coffeeproyect/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const guestCartTTL = 30 * 24 * time.Hour

type CartStore struct {
	client *redis.Client
}

func NewCartStore(client *redis.Client) *CartStore {
	return &CartStore{client: client}
}

func NewClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}

func (s *CartStore) GetItems(sessionID string) ([]domain.CartItem, error) {
	raw, err := s.client.Get(context.Background(), key(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return []domain.CartItem{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get cart: %w", err)
	}

	var items []domain.CartItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("unmarshal cart: %w", err)
	}
	return items, nil
}

func (s *CartStore) AddItem(sessionID string, item domain.CartItem) error {
	items, err := s.GetItems(sessionID)
	if err != nil {
		return err
	}

	found := false
	for i, it := range items {
		if it.CoffeeID == item.CoffeeID {
			items[i].Quantity += item.Quantity
			items[i].UnitPriceCents = item.UnitPriceCents
			found = true
			break
		}
	}
	if !found {
		items = append(items, item)
	}

	return s.save(sessionID, items)
}

func (s *CartStore) SetQuantity(sessionID, coffeeID string, quantity int) error {
	items, err := s.GetItems(sessionID)
	if err != nil {
		return err
	}

	for i, it := range items {
		if it.CoffeeID == coffeeID {
			items[i].Quantity = quantity
			return s.save(sessionID, items)
		}
	}
	return domain.ErrNotFound
}

func (s *CartStore) RemoveItem(sessionID, coffeeID string) error {
	items, err := s.GetItems(sessionID)
	if err != nil {
		return err
	}

	for i, it := range items {
		if it.CoffeeID == coffeeID {
			items = append(items[:i], items[i+1:]...)
			return s.save(sessionID, items)
		}
	}
	return domain.ErrNotFound
}

func (s *CartStore) Clear(sessionID string) error {
	return s.client.Del(context.Background(), key(sessionID)).Err()
}

func (s *CartStore) save(sessionID string, items []domain.CartItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), key(sessionID), data, guestCartTTL).Err()
}

func key(sessionID string) string {
	return "cart:guest:" + sessionID
}
