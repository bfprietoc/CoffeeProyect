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

const (
	detailTTL = 10 * time.Minute
	listTTL   = 5 * time.Minute
)

type CoffeeCache struct {
	client *redis.Client
}

func NewCoffeeCache(client *redis.Client) *CoffeeCache {
	return &CoffeeCache{client: client}
}

func (c *CoffeeCache) GetByID(id string) (domain.Coffee, bool) {
	raw, err := c.client.Get(context.Background(), detailKey(id)).Bytes()
	if errors.Is(err, redis.Nil) || err != nil {
		return domain.Coffee{}, false
	}
	var coffee domain.Coffee
	if err := json.Unmarshal(raw, &coffee); err != nil {
		return domain.Coffee{}, false
	}
	return coffee, true
}

func (c *CoffeeCache) SetByID(id string, coffee domain.Coffee) {
	data, err := json.Marshal(coffee)
	if err != nil {
		return
	}
	c.client.Set(context.Background(), detailKey(id), data, detailTTL)
}

func (c *CoffeeCache) GetList(key string) ([]domain.Coffee, bool) {
	raw, err := c.client.Get(context.Background(), listKey(key)).Bytes()
	if errors.Is(err, redis.Nil) || err != nil {
		return nil, false
	}
	var coffees []domain.Coffee
	if err := json.Unmarshal(raw, &coffees); err != nil {
		return nil, false
	}
	return coffees, true
}

func (c *CoffeeCache) SetList(key string, coffees []domain.Coffee) {
	data, err := json.Marshal(coffees)
	if err != nil {
		return
	}
	c.client.Set(context.Background(), listKey(key), data, listTTL)
}

func (c *CoffeeCache) Invalidate(id string) {
	ctx := context.Background()
	c.client.Del(ctx, detailKey(id))
	c.deleteLists(ctx)
}

func (c *CoffeeCache) InvalidateAll() {
	ctx := context.Background()
	for _, pattern := range []string{"cache:coffee:*", "cache:coffees:*"} {
		keys, err := c.client.Keys(ctx, pattern).Result()
		if err != nil || len(keys) == 0 {
			continue
		}
		c.client.Del(ctx, keys...)
	}
}

func (c *CoffeeCache) deleteLists(ctx context.Context) {
	keys, err := c.client.Keys(ctx, "cache:coffees:*").Result()
	if err != nil || len(keys) == 0 {
		return
	}
	c.client.Del(ctx, keys...)
}

func detailKey(id string) string {
	return "cache:coffee:" + id
}

func listKey(key string) string {
	return fmt.Sprintf("cache:coffees:%s", key)
}
