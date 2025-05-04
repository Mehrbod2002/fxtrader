package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeService interface {
	PlaceTrade(userID, symbolName string, tradeType models.TradeType, leverage int, volume, entryPrice float64) (*models.TradeHistory, error)
	GetTrade(id string) (*models.TradeHistory, error)
	GetTradesByUserID(userID string) ([]*models.TradeHistory, error)
	GetAllTrades() ([]*models.TradeHistory, error)
	HandleTradeResponse(response TradeResponse) error
	StartUDPListener() error
	RequestBalance(userID string) (float64, error)
}

type tradeService struct {
	tradeRepo     repository.TradeRepository
	symbolRepo    repository.SymbolRepository
	logService    LogService
	udpConn       *net.UDPConn
	mt5UDPAddr    *net.UDPAddr
	listenUDPAddr *net.UDPAddr
	responseChan  chan TradeResponse
	balanceChan   chan BalanceResponse
}

type BalanceResponse struct {
	Type      string  `json:"type"`
	UserID    string  `json:"user_id"`
	Balance   float64 `json:"balance"`
	Error     string  `json:"error,omitempty"`
	Timestamp int64   `json:"timestamp"`
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
		balanceChan:   make(chan BalanceResponse, 100),
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
				continue
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}

			if msgType, ok := msg["type"].(string); ok {
				if msgType == "trade_response" {
					var response TradeResponse
					if err := json.Unmarshal(buf[:n], &response); err != nil {
						continue
					}
					s.responseChan <- response
				} else if msgType == "balance_response" {
					var response BalanceResponse
					if err := json.Unmarshal(buf[:n], &response); err != nil {
						continue
					}
					s.balanceChan <- response
				}
			}
		}
	}()

	go func() {
		for response := range s.responseChan {
			if err := s.HandleTradeResponse(response); err != nil {
				log.Printf("Failed to handle trade response: %v", err)
			}
		}
	}()

	go func() {
		for response := range s.balanceChan {
			if err := s.handleBalanceResponse(response); err != nil {
				log.Printf("Failed to handle balance response: %v", err)
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

func (s *tradeService) RequestBalance(userID string) (float64, error) {
	_, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return 0, errors.New("invalid user ID")
	}

	balanceRequest := map[string]interface{}{
		"type":      "balance_request",
		"user_id":   userID,
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(balanceRequest)
	if err != nil {
		return 0, err
	}

	conn, err := net.DialUDP("udp", nil, s.mt5UDPAddr)
	if err != nil {
		return 0, errors.New("failed to connect to MT5 UDP")
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return 0, errors.New("failed to send balance request")
	}

	select {
	case response := <-s.balanceChan:
		if response.UserID != userID {
			return 0, errors.New("received balance response for wrong user")
		}
		if response.Error != "" {
			return 0, errors.New(response.Error)
		}
		return response.Balance, nil
	case <-time.After(5 * time.Second):
		return 0, errors.New("timeout waiting for balance response")
	}
}

func (s *tradeService) handleBalanceResponse(response BalanceResponse) error {
	userObjID, err := primitive.ObjectIDFromHex(response.UserID)
	if err != nil {
		return errors.New("invalid user ID in balance response")
	}

	metadata := map[string]interface{}{
		"user_id": response.UserID,
		"balance": response.Balance,
	}
	s.logService.LogAction(userObjID, "BalanceUpdate", "User balance updated from MT5", "", metadata)

	return nil
}
