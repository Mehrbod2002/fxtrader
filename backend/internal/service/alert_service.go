package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlertService interface {
	CreateAlert(userID string, alert *models.Alert) error
	GetAlert(id string) (*models.Alert, error)
	GetAlertsByUserID(userID string) ([]*models.Alert, error)
	ProcessPriceForAlerts(price *models.PriceData) error
	ProcessTimeBasedAlerts() error
}

type alertService struct {
	alertRepo  repository.AlertRepository
	symbolRepo repository.SymbolRepository
	logService LogService
	notifyFunc func(userID, message string) error
}

func NewAlertService(alertRepo repository.AlertRepository, symbolRepo repository.SymbolRepository, logService LogService) AlertService {
	return &alertService{
		alertRepo:  alertRepo,
		symbolRepo: symbolRepo,
		logService: logService,
		notifyFunc: func(userID, message string) error { return nil },
	}
}

func (s *alertService) CreateAlert(userID string, alert *models.Alert) error {
	if alert.AlertType != models.AlertTypePrice && alert.AlertType != models.AlertTypeTime {
		return errors.New("invalid alert type")
	}
	if alert.AlertType == models.AlertTypePrice {
		if alert.Condition.PriceTarget == nil || *alert.Condition.PriceTarget <= 0 {
			return errors.New("price target required and must be positive")
		}
		if alert.Condition.SL == nil && alert.Condition.TP == nil {
			return errors.New("comparison must be ABOVE or BELOW")
		}
	} else if alert.AlertType == models.AlertTypeTime {
		if alert.Condition.TriggerTime == nil || alert.Condition.TriggerTime.Before(time.Now()) {
			return errors.New("trigger time required and must be in the future")
		}
	}

	symbols, err := s.symbolRepo.GetAllSymbols()
	if err != nil {
		return errors.New("failed to fetch symbols")
	}
	var symbolExists bool
	for _, sym := range symbols {
		if sym.SymbolName == alert.SymbolName {
			symbolExists = true
			break
		}
	}
	if !symbolExists {
		return errors.New("symbol not found")
	}

	alert.UserID = userID
	alert.Status = models.AlertStatusPending

	err = s.alertRepo.SaveAlert(alert)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"alert_id":    alert.ID.Hex(),
		"symbol_name": alert.SymbolName,
		"alert_type":  alert.AlertType,
	}
	s.logService.LogAction(primitive.ObjectID{}, "CreateAlert", "Alert created", "", metadata)

	return nil
}

func (s *alertService) GetAlert(id string) (*models.Alert, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid alert ID")
	}
	return s.alertRepo.GetAlertByID(objID)
}

func (s *alertService) GetAlertsByUserID(userID string) ([]*models.Alert, error) {
	return s.alertRepo.GetAlertsByUserID(userID)
}

func (s *alertService) ProcessPriceForAlerts(price *models.PriceData) error {
	alerts, err := s.alertRepo.GetPendingAlerts()
	if err != nil {
		return err
	}

	for _, alert := range alerts {
		if alert.SymbolName != price.Symbol || alert.AlertType != models.AlertTypePrice {
			continue
		}

		shouldTrigger := false
		if price.Ask <= *alert.Condition.SL && price.Ask >= *alert.Condition.PriceTarget {
			shouldTrigger = true
		}

		if price.Bid >= *alert.Condition.TP && price.Bid <= *alert.Condition.PriceTarget {
			shouldTrigger = true
		}

		if shouldTrigger {
			now := time.Now()
			alert.Status = models.AlertStatusTriggered
			alert.TriggeredAt = &now
			err = s.alertRepo.UpdateAlert(alert.ID, alert)
			if err != nil {
				continue
			}

			message := "Alert triggered for " + alert.SymbolName + " at price " + fmt.Sprintf("%f", *alert.Condition.PriceTarget)
			s.notifyFunc(alert.UserID, message)

			metadata := map[string]interface{}{
				"alert_id":     alert.ID.Hex(),
				"symbol_name":  alert.SymbolName,
				"price_target": *alert.Condition.PriceTarget,
			}
			s.logService.LogAction(primitive.ObjectID{}, "AlertTriggered", "Price alert triggered", "", metadata)
		}
	}

	return nil
}

func (s *alertService) ProcessTimeBasedAlerts() error {
	alerts, err := s.alertRepo.GetPendingAlerts()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, alert := range alerts {
		if alert.AlertType != models.AlertTypeTime || alert.Condition.TriggerTime == nil {
			continue
		}

		if now.After(*alert.Condition.TriggerTime) {
			alert.Status = models.AlertStatusTriggered
			alert.TriggeredAt = &now
			err = s.alertRepo.UpdateAlert(alert.ID, alert)
			if err != nil {
				continue
			}

			message := "Time-based alert triggered for " + alert.SymbolName
			s.notifyFunc(alert.UserID, message)

			metadata := map[string]interface{}{
				"alert_id":    alert.ID.Hex(),
				"symbol_name": alert.SymbolName,
			}
			s.logService.LogAction(primitive.ObjectID{}, "AlertTriggered", "Time alert triggered", "", metadata)
		}
	}

	return nil
}
