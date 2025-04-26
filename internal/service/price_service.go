package service

import (
	"fxtrader/internal/models"
	"fxtrader/internal/repository"
)

type PriceService interface {
	ProcessPrice(data *models.PriceData) error
}

type priceService struct {
	repo repository.PriceRepository
}

func NewPriceService(repo repository.PriceRepository) PriceService {
	return &priceService{repo: repo}
}

func (s *priceService) ProcessPrice(data *models.PriceData) error {
	return s.repo.SavePrice(data)
}
