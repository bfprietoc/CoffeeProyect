package cache

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"fmt"
)

type CoffeeCache interface {
	GetByID(id string) (domain.Coffee, bool)
	SetByID(id string, coffee domain.Coffee)
	GetList(key string) ([]domain.Coffee, bool)
	SetList(key string, coffees []domain.Coffee)
	// Invalidate removes the detail entry for id and all list caches.
	// Called when stock or product data changes.
	Invalidate(id string)
	InvalidateAll()
}

// ListKey returns a stable cache key for a given filter combination.
func ListKey(f store.CoffeeFilters) string {
	avail := ""
	if f.Available != nil {
		if *f.Available {
			avail = "1"
		} else {
			avail = "0"
		}
	}
	return fmt.Sprintf("roast=%s:country=%s:process=%s:avail=%s:page=%d:limit=%d",
		f.RoastLevel, f.Country, f.Process, avail, f.Page, f.Limit)
}
