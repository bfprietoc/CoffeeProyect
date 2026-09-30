package memory

import (
	"coffeeproyect/internal/domain"
	"sync"
)

type CoffeeCache struct {
	mu    sync.RWMutex
	byID  map[string]domain.Coffee
	lists map[string][]domain.Coffee
}

func NewCoffeeCache() *CoffeeCache {
	return &CoffeeCache{
		byID:  make(map[string]domain.Coffee),
		lists: make(map[string][]domain.Coffee),
	}
}

func (c *CoffeeCache) GetByID(id string) (domain.Coffee, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	coffee, ok := c.byID[id]
	return coffee, ok
}

func (c *CoffeeCache) SetByID(id string, coffee domain.Coffee) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[id] = coffee
}

func (c *CoffeeCache) GetList(key string) ([]domain.Coffee, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	coffees, ok := c.lists[key]
	return coffees, ok
}

func (c *CoffeeCache) SetList(key string, coffees []domain.Coffee) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lists[key] = coffees
}

func (c *CoffeeCache) Invalidate(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.byID, id)
	c.lists = make(map[string][]domain.Coffee)
}

func (c *CoffeeCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID = make(map[string]domain.Coffee)
	c.lists = make(map[string][]domain.Coffee)
}
