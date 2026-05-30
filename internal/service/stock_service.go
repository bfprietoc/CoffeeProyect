package service

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/store"
)

type StockService struct {
	stockStore    store.StockStore
	coffeeService *CoffeeService // for cache invalidation
	publisher     event.Publisher
}

func NewStockService(s store.StockStore, coffee *CoffeeService, p event.Publisher) *StockService {
	return &StockService{stockStore: s, coffeeService: coffee, publisher: p}
}

func (s *StockService) Adjust(coffeeID string, delta int, note string) (store.StockAdjustResult, error) {
	result, err := s.stockStore.Adjust(coffeeID, delta, note)
	if err != nil {
		return store.StockAdjustResult{}, err
	}

	// invalidate catalog cache for this coffee
	if s.coffeeService != nil {
		s.coffeeService.InvalidateProduct(coffeeID)
	}

	payload := event.StockPayload{
		CoffeeID:    coffeeID,
		Delta:       delta,
		OldStock:    result.OldStock,
		ResultStock: result.ResultStock,
		Note:        note,
	}

	if s.publisher != nil {
		if delta > 0 {
			_ = s.publisher.Publish(event.TopicStockReplenished, payload)
			if result.OldStock == 0 {
				_ = s.publisher.Publish(event.TopicStockRestored, payload)
			}
		} else {
			_ = s.publisher.Publish(event.TopicStockDecremented, payload)
			if result.ResultStock == 0 {
				_ = s.publisher.Publish(event.TopicStockDepleted, payload)
			}
		}
	}

	return result, nil
}

func (s *StockService) GetStock(coffeeID string) (int, error) {
	return s.stockStore.GetStock(coffeeID)
}

func (s *StockService) GetMovements(coffeeID string) ([]domain.StockMovement, error) {
	return s.stockStore.GetMovements(coffeeID)
}
