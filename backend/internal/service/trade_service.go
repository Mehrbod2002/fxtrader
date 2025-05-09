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
	PlaceTrade(userID, symbol string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, error)
	GetTrade(id string) (*models.TradeHistory, error)
	GetTradesByUserID(userID string) ([]*models.TradeHistory, error)
	GetAllTrades() ([]*models.TradeHistory, error)
	HandleTradeResponse(response TradeResponse) error
	HandleTradeRequest(request map[string]interface{}) error
	HandleBalanceRequest(request map[string]interface{}) error
	RequestBalance(userID string) (float64, error)
}

type tradeService struct {
	tradeRepo        repository.TradeRepository
	symbolRepo       repository.SymbolRepository
	logService       LogService
	mt5TCPAddr       *net.TCPAddr
	responseChan     chan TradeResponse
	balanceChan      chan BalanceResponse
	copyTradeService CopyTradeService
}

type BalanceResponse struct {
	Type      string  `json:"type"`
	UserID    string  `json:"user_id"`
	Balance   float64 `json:"balance"`
	Error     string  `json:"error,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

func NewTradeService(tradeRepo repository.TradeRepository, symbolRepo repository.SymbolRepository, logService LogService, copyTradeService CopyTradeService, mt5Host string, mt5Port int) (TradeService, error) {
	mt5Addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", mt5Host, mt5Port))
	if err != nil {
		return nil, err
	}

	return &tradeService{
		tradeRepo:        tradeRepo,
		symbolRepo:       symbolRepo,
		logService:       logService,
		mt5TCPAddr:       mt5Addr,
		responseChan:     make(chan TradeResponse, 100),
		balanceChan:      make(chan BalanceResponse, 100),
		copyTradeService: copyTradeService,
	}, nil
}

func (s *tradeService) HandleTradeRequest(request map[string]interface{}) error {
	_, ok := request["trade_id"].(string)
	if !ok {
		return errors.New("missing or invalid trade_id")
	}
	userID, ok := request["user_id"].(string)
	if !ok {
		return errors.New("missing or invalid user_id")
	}
	symbol, ok := request["symbol"].(string)
	if !ok {
		return errors.New("missing or invalid symbol")
	}
	tradeTypeStr, ok := request["trade_type"].(string)
	if !ok {
		return errors.New("missing or invalid trade_type")
	}
	orderType, ok := request["order_type"].(string)
	if !ok {
		return errors.New("missing or invalid order_type")
	}
	leverage, ok := request["leverage"].(float64)
	if !ok {
		return errors.New("missing or invalid leverage")
	}
	volume, ok := request["volume"].(float64)
	if !ok {
		return errors.New("missing or invalid volume")
	}
	entryPrice, ok := request["entry_price"].(float64)
	if !ok {
		return errors.New("missing or invalid entry_price")
	}
	stopLoss, ok := request["stop_loss"].(float64)
	if !ok {
		return errors.New("missing or invalid stop_loss")
	}
	takeProfit, ok := request["take_profit"].(float64)
	if !ok {
		return errors.New("missing or invalid take_profit")
	}
	var expiration *time.Time
	if exp, ok := request["expiration"].(float64); ok && exp > 0 {
		t := time.Unix(int64(exp), 0)
		expiration = &t
	}

	tradeType := models.TradeType(tradeTypeStr)
	_, err := s.PlaceTrade(userID, symbol, tradeType, orderType, int(leverage), volume, entryPrice, stopLoss, takeProfit, expiration)
	return err
}

func (s *tradeService) HandleBalanceRequest(request map[string]interface{}) error {
	userID, ok := request["user_id"].(string)
	if !ok {
		return errors.New("missing or invalid user_id")
	}

	_, err := s.RequestBalance(userID)
	return err
}

func (s *tradeService) sendToMT5(data []byte) error {
	conn, err := net.DialTCP("tcp", nil, s.mt5TCPAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to MT5 TCP server: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send data to MT5: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read MT5 response: %v", err)
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		return fmt.Errorf("failed to unmarshal MT5 response: %v", err)
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return errors.New("missing or invalid response type")
	}

	if msgType == "trade_response" {
		var response TradeResponse
		if err := json.Unmarshal(buf[:n], &response); err != nil {
			return fmt.Errorf("failed to unmarshal trade response: %v", err)
		}
		s.responseChan <- response
	} else if msgType == "balance_response" {
		var response BalanceResponse
		if err := json.Unmarshal(buf[:n], &response); err != nil {
			return fmt.Errorf("failed to unmarshal balance response: %v", err)
		}
		s.balanceChan <- response
	}

	return nil
}

func (s *tradeService) PlaceTrade(userID, symbolName string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, error) {
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

	validOrderTypes := []string{"MARKET", "LIMIT", "BUY_STOP", "SELL_STOP", "BUY_LIMIT", "SELL_LIMIT"}
	isValidOrderType := false
	for _, ot := range validOrderTypes {
		if orderType == ot {
			isValidOrderType = true
			break
		}
	}
	if !isValidOrderType {
		return nil, errors.New("invalid order type")
	}

	if volume < symbol.MinLot || volume > symbol.MaxLot {
		return nil, errors.New("volume out of allowed range")
	}

	if leverage > symbol.Leverage {
		return nil, errors.New("leverage exceeds symbol limit")
	}

	if orderType != "MARKET" && entryPrice <= 0 {
		return nil, errors.New("entry price required for non-market orders")
	}
	if orderType == "MARKET" && entryPrice > 0 {
		return nil, errors.New("entry price not allowed for market orders")
	}

	if stopLoss < 0 || takeProfit < 0 {
		return nil, errors.New("stop loss and take profit cannot be negative")
	}

	if expiration != nil && expiration.Before(time.Now()) {
		return nil, errors.New("expiration time must be in the future")
	}

	trade := &models.TradeHistory{
		ID:         primitive.NewObjectID(),
		UserID:     userObjID,
		Symbol:     symbolName,
		TradeType:  tradeType,
		OrderType:  orderType,
		Leverage:   leverage,
		Volume:     volume,
		EntryPrice: entryPrice,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
		OpenTime:   time.Now(),
		Status:     string(models.TradeStatusPending),
		Expiration: expiration,
	}

	tradeRequest := map[string]interface{}{
		"type":        "trade_request",
		"trade_id":    trade.ID.Hex(),
		"user_id":     trade.UserID.Hex(),
		"symbol":      trade.Symbol,
		"trade_type":  trade.TradeType,
		"order_type":  trade.OrderType,
		"leverage":    trade.Leverage,
		"volume":      trade.Volume,
		"entry_price": trade.EntryPrice,
		"stop_loss":   trade.StopLoss,
		"take_profit": trade.TakeProfit,
		"timestamp":   trade.OpenTime.Unix(),
		"expiration":  0,
	}
	if trade.Expiration != nil {
		tradeRequest["expiration"] = trade.Expiration.Unix()
	}

	data, err := json.Marshal(tradeRequest)
	if err != nil {
		return nil, err
	}

	if err := s.sendToMT5(data); err != nil {
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
		"order_type":  orderType,
		"leverage":    leverage,
		"volume":      volume,
		"entry_price": entryPrice,
		"stop_loss":   stopLoss,
		"take_profit": takeProfit,
		"expiration":  expiration,
	}
	s.logService.LogAction(userObjID, "PlaceTrade", "Trade order placed", "", metadata)

	go func() {
		if err := s.copyTradeService.MirrorTrade(trade); err != nil {
			log.Printf("Failed to mirror trade: %v", err)
		}
	}()

	return trade, nil
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
		trade.Status = string(models.TradeStatusOpen)
		trade.MatchedTradeID = response.MatchedTradeID
	} else if response.Status == "PENDING" {
		trade.Status = string(models.TradeStatusPending)
	} else if response.Status == "EXPIRED" {
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
	} else {
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
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

	if err := s.sendToMT5(data); err != nil {
		return 0, fmt.Errorf("failed to send balance request: %v", err)
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

type TradeResponse struct {
	TradeID        string `json:"trade_id"`
	UserID         string `json:"user_id"`
	Status         string `json:"status"`
	MatchedTradeID string `json:"matched_trade_id"`
	Timestamp      int64  `json:"timestamp"`
}
