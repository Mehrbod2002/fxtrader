package service

import (
	"log"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/ws"
)

type PriceService interface {
	ProcessPrice(data *models.PriceData) error
}

type priceService struct {
	repo repository.PriceRepository
	hub  *ws.Hub
}

func NewPriceService(repo repository.PriceRepository, hub *ws.Hub) PriceService {
	return &priceService{
		repo: repo,
		hub:  hub,
	}
}

func (s *priceService) ProcessPrice(data *models.PriceData) error {
	if err := s.repo.SavePrice(data); err != nil {
		return err
	}

	s.hub.BroadcastPrice(data)
	log.Printf("Price broadcast: %s Ask: %.5f Bid: %.5f", data.Symbol, data.Ask, data.Bid)

	return nil
}
