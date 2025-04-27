package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"fxtrader/internal/models"
	"fxtrader/internal/repository"
	"log"
	"net"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeService interface {
	PlaceTrade(userID, symbolName string, tradeType models.TradeType, leverage int, volume, entryPrice float64) (*models.TradeHistory, error)
	GetTrade(id string) (*models.TradeHistory, error)
	GetTradesByUserID(userID string) ([]*models.TradeHistory, error)
	GetAllTrades() ([]*models.TradeHistory, error)
	HandleTradeResponse(response TradeResponse) error
	StartUDPListener() error
}

type tradeService struct {
	tradeRepo     repository.TradeRepository
	symbolRepo    repository.SymbolRepository
	logService    LogService
	udpConn       *net.UDPConn
	mt5UDPAddr    *net.UDPAddr
	listenUDPAddr *net.UDPAddr
	responseChan  chan TradeResponse
}

func NewTradeService(tradeRepo repository.TradeRepository, symbolRepo repository.SymbolRepository, logService LogService, mt5Host string, mt5Port, listenPort int) (TradeService, error) {
	mt5Addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", mt5Host, mt5Port))
	if err != nil {
		return nil, err
	}
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, err
	}

	return &tradeService{
		tradeRepo:     tradeRepo,
		symbolRepo:    symbolRepo,
		logService:    logService,
		mt5UDPAddr:    mt5Addr,
		listenUDPAddr: listenAddr,
		responseChan:  make(chan TradeResponse, 100),
	}, nil
}

func (s *tradeService) StartUDPListener() error {
	conn, err := net.ListenUDP("udp", s.listenUDPAddr)
	if err != nil {
		return err
	}
	s.udpConn = conn

	go func() {
		defer conn.Close()
		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("UDP read error: %v", err)
				continue
			}

			var response TradeResponse
			if err := json.Unmarshal(buf[:n], &response); err != nil {
				log.Printf("Failed to parse UDP response: %v", err)
				continue
			}

			s.responseChan <- response
		}
	}()

	go func() {
		for response := range s.responseChan {
			if err := s.HandleTradeResponse(response); err != nil {
				log.Printf("Failed to handle trade response: %v", err)
			}
		}
	}()

	return nil
}

func (s *tradeService) PlaceTrade(userID, symbolName string, tradeType models.TradeType, leverage int, volume, entryPrice float64) (*models.TradeHistory, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	symbols, err := s.symbolRepo.GetAllSymbols()
	if err != nil {
		return nil, errors.New("failed to fetch symbols")
	}
	var symbol *models.Symbol
	for _, sym := range symbols {
		if sym.SymbolName == symbolName {
			symbol = sym
			break
		}
	}
	if symbol == nil {
		return nil, errors.New("symbol not found")
	}

	if tradeType != models.TradeTypeBuy && tradeType != models.TradeTypeSell {
		return nil, errors.New("invalid trade type")
	}
	if volume < symbol.MinLot || volume > symbol.MaxLot {
		return nil, errors.New("volume out of allowed range")
	}
	if leverage > symbol.Leverage {
		return nil, errors.New("leverage exceeds symbol limit")
	}

	trade := &models.TradeHistory{
		ID:         primitive.NewObjectID(),
		UserID:     userObjID,
		SymbolName: symbolName,
		TradeType:  tradeType,
		Leverage:   leverage,
		Volume:     volume,
		EntryPrice: entryPrice,
		OpenTime:   time.Now(),
		Status:     models.TradeStatusPending,
	}

	err = s.sendTradeToMT5(trade)
	if err != nil {
		return nil, err
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return nil, err
	}

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

	conn, err := net.DialUDP("udp", nil, s.mt5UDPAddr)
	if err != nil {
		return errors.New("failed to connect to MT5 UDP")
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return errors.New("failed to send trade request")
	}

	return nil
}

func (s *tradeService) HandleTradeResponse(response TradeResponse) error {
	tradeID, err := primitive.ObjectIDFromHex(response.TradeID)
	if err != nil {
		return errors.New("invalid trade ID")
	}

	trade, err := s.tradeRepo.GetTradeByID(tradeID)
	if err != nil {
		return err
	}
	if trade == nil {
		return errors.New("trade not found")
	}

	if response.Status == "MATCHED" {
		trade.Status = models.TradeStatusOpen
		trade.ID, _ = primitive.ObjectIDFromHex(response.MatchedTradeID)
	} else if response.Status == "PENDING" {
		trade.Status = models.TradeStatusPending
	} else {
		trade.Status = models.TradeStatusClosed
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"trade_id":         response.TradeID,
		"status":           response.Status,
		"matched_trade_id": response.MatchedTradeID,
	}
	s.logService.LogAction(trade.UserID, "TradeResponse", "Trade status updated", "", metadata)

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

func (s *tradeService) GetAllTrades() ([]*models.TradeHistory, error) {
	return s.tradeRepo.GetAllTrades()
}

type TradeResponse struct {
	TradeID        string `json:"trade_id"`
	UserID         string `json:"user_id"`
	Status         string `json:"status"`
	MatchedTradeID string `json:"matched_trade_id"`
	Timestamp      int64  `json:"timestamp"`
}
