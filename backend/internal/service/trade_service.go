package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fxtrader/internal/models"
	"fxtrader/internal/repository"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeService interface {
	PlaceTrade(userID, symbolName string, tradeType models.TradeType, leverage int, volume, entryPrice float64) (*models.TradeHistory, error)
	GetTrade(id string) (*models.TradeHistory, error)
	GetTradesByUserID(userID string) ([]*models.TradeHistory, error)
}

type tradeService struct {
	tradeRepo   repository.TradeRepository
	symbolRepo  repository.SymbolRepository
	logService  LogService
	mt5Endpoint string // URL for MT5 trade API
}

func NewTradeService(tradeRepo repository.TradeRepository, symbolRepo repository.SymbolRepository, logService LogService, mt5Endpoint string) TradeService {
	return &tradeService{
		tradeRepo:   tradeRepo,
		symbolRepo:  symbolRepo,
		logService:  logService,
		mt5Endpoint: mt5Endpoint, // e.g., "http://mt5-server:8080/trade"
	}
}

func (s *tradeService) PlaceTrade(userID, symbolName string, tradeType models.TradeType, leverage int, volume, entryPrice float64) (*models.TradeHistory, error) {
	// Validate user ID
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	// Check if symbol exists
	symbols, err := s.symbolRepo.GetAllSymbols()
	if err != nil {
		return nil, errors.New("failed to fetch symbols")
	}
	var symbol *models.Symbol
	for _, s := range symbols {
		if s.SymbolName == symbolName {
			symbol = s
			break
		}
	}
	if symbol == nil {
		return nil, errors.New("symbol not found")
	}

	// Validate trade parameters
	if tradeType != models.TradeTypeBuy && tradeType != models.TradeTypeSell {
		return nil, errors.New("invalid trade type")
	}
	if volume < symbol.MinLot || volume > symbol.MaxLot {
		return nil, errors.New("volume out of allowed range")
	}
	if leverage > symbol.Leverage {
		return nil, errors.New("leverage exceeds symbol limit")
	}

	// Create trade record
	trade := &models.TradeHistory{
		UserID:     userObjID,
		SymbolName: symbolName,
		TradeType:  tradeType,
		Leverage:   leverage,
		Volume:     volume,
		EntryPrice: entryPrice,
		Status:     "PENDING",
	}

	// Send trade to MT5
	err = s.sendTradeToMT5(trade)
	if err != nil {
		return nil, err
	}

	// Save trade to database
	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return nil, err
	}

	// Log the action
	metadata := map[string]interface{}{
		"trade_id":    trade.ID.Hex(),
		"symbol_name": symbolName,
		"trade_type":  tradeType,
		"leverage":    leverage,
		"volume":      volume,
		"entry_price": entryPrice,
	}
	s.logService.LogAction(userObjID, "PlaceTrade", "Trade order placed", "", metadata)

	return trade, nil
}

func (s *tradeService) sendTradeToMT5(trade *models.TradeHistory) error {
	// Prepare trade request for MT5
	tradeRequest := map[string]interface{}{
		"trade_id":    trade.ID.Hex(),
		"user_id":     trade.UserID.Hex(),
		"symbol":      trade.SymbolName,
		"trade_type":  trade.TradeType,
		"leverage":    trade.Leverage,
		"volume":      trade.Volume,
		"entry_price": trade.EntryPrice,
		"timestamp":   trade.OpenTime.Unix(),
	}

	data, err := json.Marshal(tradeRequest)
	if err != nil {
		return err
	}

	// Send to MT5 (replace with actual MT5 API endpoint)
	resp, err := http.Post(s.mt5Endpoint, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to send trade to MT5")
	}

	// Update trade status based on MT5 response (simplified)
	trade.Status = "OPEN"
	return nil
}

func (s *tradeService) GetTrade(id string) (*models.TradeHistory, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.tradeRepo.GetTradeByID(objID)
}

func (s *tradeService) GetTradesByUserID(userID string) ([]*models.TradeHistory, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	return s.tradeRepo.GetTradesByUserID(objID)
}
