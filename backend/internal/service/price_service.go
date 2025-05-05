package service

import (
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/ws"
)

type PriceService interface {
	ProcessPrice(data *models.PriceData) error
}

type priceService struct {
	repo         repository.PriceRepository
	hub          *ws.Hub
	alertService AlertService
}

func NewPriceService(repo repository.PriceRepository, hub *ws.Hub, alertService AlertService) PriceService {
	return &priceService{
		repo:         repo,
		hub:          hub,
		alertService: alertService,
	}
}

func (s *priceService) ProcessPrice(data *models.PriceData) error {
	if err := s.repo.SavePrice(data); err != nil {
		return err
	}

	s.hub.BroadcastPrice(data)

	if err := s.alertService.ProcessPriceForAlerts(data); err != nil {
		return err
	}

	return nil
}
