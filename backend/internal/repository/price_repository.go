package repository

import (
	"fxtrader/internal/models"
	"sync"
)

type PriceRepository interface {
	SavePrice(data *models.PriceData) error
}

type InMemoryPriceRepository struct {
	prices []*models.PriceData
	mu     sync.Mutex
}

func NewPriceRepository() PriceRepository {
	return &InMemoryPriceRepository{
		prices: make([]*models.PriceData, 0),
	}
}

func (r *InMemoryPriceRepository) SavePrice(data *models.PriceData) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices = append(r.prices, data)
	return nil
}
