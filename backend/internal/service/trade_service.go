package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/ws"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	mt5ReconnectBackoffInitial = 2 * time.Second
	mt5ReconnectBackoffMax     = 30 * time.Second
	mt5ReconnectMaxAttempts    = 5
)

type TradeService interface {
	PlaceTrade(userID, symbol, accountType string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, error)
	CloseTrade(tradeID, userID, accountType string) error
	StreamTrades(userID, accountType string) error
	GetTrade(id string) (*models.TradeHistory, error)
	GetTradesByUserID(userID string) ([]*models.TradeHistory, error)
	GetAllTrades() ([]*models.TradeHistory, error)
	HandleTradeResponse(response TradeResponse) error
	HandleCloseTradeResponse(response CloseTradeResponse) error
	HandleOrderStreamResponse(response OrderStreamResponse) error
	HandleTradeRequest(request map[string]interface{}) error
	HandleBalanceRequest(request map[string]interface{}) error
	RequestBalance(userID, accountType string) (float64, error)
	RegisterMT5Connection(conn *websocket.Conn)
}

type tradeService struct {
	tradeRepo          repository.TradeRepository
	symbolRepo         repository.SymbolRepository
	userRepo           repository.UserRepository
	logService         LogService
	mt5Conn            *websocket.Conn
	mt5ConnMu          sync.Mutex
	responseChan       chan interface{}
	balanceChan        chan BalanceResponse
	hub                *ws.Hub
	copyTradeService   CopyTradeService
	tradeResponseChans map[string]chan TradeResponse
	tradeResponseMu    sync.Mutex
}

type CloseTradeResponse struct {
	TradeID     string  `json:"trade_id"`
	UserID      string  `json:"user_id"`
	AccountType string  `json:"account_type"` // "DEMO" or "REAL"
	Status      string  `json:"status"`       // e.g., "SUCCESS", "FAILED"
	ClosePrice  float64 `json:"close_price"`
	CloseReason string  `json:"close_reason"`
	Timestamp   float64 `json:"timestamp"`
}

type OrderStreamResponse struct {
	UserID      string                `json:"user_id"`
	AccountType string                `json:"account_type"` // "DEMO" or "REAL"
	Trades      []models.TradeHistory `json:"trades"`
	Timestamp   float64               `json:"timestamp"`
}

type BalanceResponse struct {
	Type        string  `json:"type"`
	UserID      string  `json:"user_id"`
	AccountType string  `json:"account_type"` // "DEMO" or "REAL"
	Balance     float64 `json:"balance"`
	Error       string  `json:"error,omitempty"`
	Timestamp   float64 `json:"timestamp"`
}

type TradeResponse struct {
	TradeID        string  `json:"trade_id"`
	UserID         string  `json:"user_id"`
	Status         string  `json:"status"`
	MatchedTradeID string  `json:"matched_trade_id"`
	Timestamp      float64 `json:"timestamp"`
}

func NewTradeService(tradeRepo repository.TradeRepository, symbolRepo repository.SymbolRepository, userRepo repository.UserRepository, logService LogService, hub *ws.Hub, copyTradeService CopyTradeService) (TradeService, error) {
	return &tradeService{
		tradeRepo:          tradeRepo,
		symbolRepo:         symbolRepo,
		userRepo:           userRepo,
		logService:         logService,
		responseChan:       make(chan interface{}, 100),
		balanceChan:        make(chan BalanceResponse, 100),
		hub:                hub,
		copyTradeService:   copyTradeService,
		tradeResponseChans: make(map[string]chan TradeResponse),
	}, nil
}

func (s *tradeService) reconnectMT5() error {
	s.mt5ConnMu.Lock()
	defer s.mt5ConnMu.Unlock()

	if s.mt5Conn != nil {
		s.mt5Conn.Close()
		s.mt5Conn = nil
	}

	backoff := mt5ReconnectBackoffInitial
	attempts := 0
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	for attempts < mt5ReconnectMaxAttempts {
		conn, _, err := dialer.Dial("ws://127.0.0.1:8080/ws", nil)
		if err != nil {
			log.Printf("Failed to reconnect to MT5: %v", err)
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * 1.5)
			if backoff > mt5ReconnectBackoffMax {
				backoff = mt5ReconnectBackoffMax
			}
			attempts++
			continue
		}

		conn.SetReadLimit(4096)
		s.mt5Conn = conn
		log.Printf("Reconnected to MT5")
		go s.RegisterMT5Connection(conn)
		return nil
	}
	return fmt.Errorf("failed to reconnect to MT5 after %d attempts", mt5ReconnectMaxAttempts)
}

func (s *tradeService) RegisterMT5Connection(conn *websocket.Conn) {
	s.mt5ConnMu.Lock()
	s.mt5Conn = conn
	s.mt5ConnMu.Unlock()

	go func() {
		for {
			if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				log.Printf("Failed to set read deadline: %v", err)
				go func() {
					if err := s.reconnectMT5(); err != nil {
						log.Printf("Failed to reconnect MT5: %v", err)
					}
				}()
				return
			}

			_, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("WebSocket closed normally")
				} else {
					log.Printf("Failed to read from MT5 connection: %v", err)
				}
				go func() {
					if err := s.reconnectMT5(); err != nil {
						log.Printf("Failed to reconnect MT5: %v", err)
					}
				}()
				return
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("Failed to unmarshal MT5 response: %v", err)
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				log.Printf("Missing or invalid response type")
				continue
			}

			if msgType == "handshake" {
				handshakeResponse := map[string]interface{}{
					"type":      "handshake_response",
					"version":   "1.0",
					"timestamp": time.Now().Unix(),
				}
				if err := s.sendToMT5(handshakeResponse); err != nil {
					log.Printf("Failed to send handshake response: %v", err)
					continue
				}
				log.Printf("Sent handshake response")
			} else if msgType == "trade_response" {
				var response TradeResponse
				if err := json.Unmarshal(data, &response); err != nil {
					log.Printf("Failed to unmarshal trade response: %v", err)
					continue
				}
				s.responseChan <- response
				s.tradeResponseMu.Lock()
				if ch, exists := s.tradeResponseChans[response.TradeID]; exists {
					select {
					case ch <- response:
					default:
						log.Printf("Trade response channel for trade %s is full or closed", response.TradeID)
					}
				}
				s.tradeResponseMu.Unlock()
			} else if msgType == "close_trade_response" {
				var response CloseTradeResponse
				if err := json.Unmarshal(data, &response); err != nil {
					log.Printf("Failed to unmarshal close trade response: %v", err)
					continue
				}
				s.responseChan <- response
			} else if msgType == "order_stream_response" {
				var response OrderStreamResponse
				if err := json.Unmarshal(data, &response); err != nil {
					log.Printf("Failed to unmarshal order stream response: %v", err)
					continue
				}
				s.responseChan <- response
			} else if msgType == "balance_response" {
				var response BalanceResponse
				if err := json.Unmarshal(data, &response); err != nil {
					log.Printf("Failed to unmarshal balance response: %v", err)
					continue
				}
				s.balanceChan <- response
			}
		}
	}()
}

func (s *tradeService) sendToMT5(msg interface{}) error {
	s.mt5ConnMu.Lock()
	defer s.mt5ConnMu.Unlock()

	if s.mt5Conn == nil {
		if err := s.reconnectMT5(); err != nil {
			return fmt.Errorf("no MT5 connection and reconnect failed: %v", err)
		}
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	if err := s.mt5Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set write deadline: %v", err)
	}

	if err := s.mt5Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send data to MT5, initiating reconnect: %v", err)
		s.mt5Conn.Close()
		s.mt5Conn = nil
		go func() {
			if err := s.reconnectMT5(); err != nil {
				log.Printf("Failed to reconnect MT5: %v", err)
			}
		}()
		return fmt.Errorf("failed to send data to MT5: %v", err)
	}
	return nil
}

func (s *tradeService) PlaceTrade(userID, symbol, accountType string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil {
		return nil, errors.New("failed to fetch user")
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	if !slices.Contains(user.AccountTypes, accountType) {
		return nil, fmt.Errorf("user does not have %s account", accountType)
	}

	var balance float64
	if accountType == "DEMO" {
		balance = user.DemoMT5Balance
	} else if accountType == "REAL" {
		balance = user.RealMT5Balance
	} else {
		return nil, errors.New("invalid account type")
	}

	if balance <= 0 {
		return nil, fmt.Errorf("insufficient %s MT5 balance", accountType)
	}

	symbols, err := s.symbolRepo.GetAllSymbols()
	if err != nil {
		return nil, errors.New("failed to fetch symbols")
	}

	var symbolObj *models.Symbol
	for _, sym := range symbols {
		if sym.SymbolName == symbol {
			symbolObj = sym
			break
		}
	}

	if symbolObj == nil {
		return nil, errors.New("symbol not found")
	}

	if tradeType != models.TradeTypeBuy && tradeType != models.TradeTypeSell {
		return nil, errors.New("invalid trade type")
	}

	validOrderTypes := []string{"MARKET", "LIMIT", "BUY_STOP", "SELL_STOP", "BUY_LIMIT", "SELL_LIMIT"}
	isValidOrderType := slices.Contains(validOrderTypes, orderType)
	if !isValidOrderType {
		return nil, errors.New("invalid order type")
	}

	if volume < symbolObj.MinLot || volume > symbolObj.MaxLot {
		return nil, errors.New("volume out of allowed range")
	}

	if leverage > symbolObj.Leverage {
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
		ID:          primitive.NewObjectID(),
		UserID:      userObjID,
		Symbol:      symbol,
		TradeType:   tradeType,
		OrderType:   orderType,
		Leverage:    leverage,
		Volume:      volume,
		EntryPrice:  entryPrice,
		StopLoss:    stopLoss,
		TakeProfit:  takeProfit,
		OpenTime:    time.Now(),
		Status:      string(models.TradeStatusPending),
		Expiration:  expiration,
		AccountType: accountType,
	}

	tradeRequest := map[string]interface{}{
		"type":         "trade_request",
		"trade_id":     trade.ID.Hex(),
		"user_id":      trade.UserID.Hex(),
		"account_type": accountType,
		"symbol":       trade.Symbol,
		"trade_type":   trade.TradeType,
		"order_type":   trade.OrderType,
		"leverage":     trade.Leverage,
		"volume":       trade.Volume,
		"entry_price":  trade.EntryPrice,
		"stop_loss":    trade.StopLoss,
		"take_profit":  trade.TakeProfit,
		"timestamp":    trade.OpenTime.Unix(),
		"expiration":   0,
	}
	if trade.Expiration != nil {
		tradeRequest["expiration"] = trade.Expiration.Unix()
	}

	responseChan := make(chan TradeResponse, 1)
	s.tradeResponseMu.Lock()
	s.tradeResponseChans[trade.ID.Hex()] = responseChan
	s.tradeResponseMu.Unlock()

	defer func() {
		s.tradeResponseMu.Lock()
		delete(s.tradeResponseChans, trade.ID.Hex())
		close(responseChan)
		s.tradeResponseMu.Unlock()
	}()

	if err := s.sendToMT5(tradeRequest); err != nil {
		return nil, err
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return nil, err
	}

	select {
	case response := <-responseChan:
		if response.TradeID != trade.ID.Hex() {
			return nil, errors.New("received response for wrong trade ID")
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
			trade.CloseReason = "EXPIRED"
		} else {
			trade.Status = string(models.TradeStatusClosed)
			trade.CloseTime = &time.Time{}
			*trade.CloseTime = time.Now()
			trade.CloseReason = response.Status
		}
		err = s.tradeRepo.SaveTrade(trade)
		if err != nil {
			return nil, err
		}
		metadata := map[string]interface{}{
			"trade_id":         response.TradeID,
			"status":           response.Status,
			"matched_trade_id": response.MatchedTradeID,
		}
		if err := s.logService.LogAction(trade.UserID, "TradeResponse", "Trade status updated", "", metadata); err != nil {
			log.Printf("error: %v", err)
		}
	case <-time.After(10 * time.Second):
		return nil, errors.New("timeout waiting for MT5 trade response")
	}

	metadata := map[string]interface{}{
		"trade_id":     trade.ID.Hex(),
		"symbol_name":  symbol,
		"account_type": accountType,
		"trade_type":   tradeType,
		"order_type":   orderType,
		"leverage":     leverage,
		"volume":       volume,
		"entry_price":  entryPrice,
		"stop_loss":    stopLoss,
		"take_profit":  takeProfit,
		"expiration":   expiration,
	}

	if err := s.logService.LogAction(userObjID, "PlaceTrade", "Trade order placed", "", metadata); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	go func() {
		if err := s.copyTradeService.MirrorTrade(trade, accountType); err != nil {
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
		trade.CloseReason = "EXPIRED"
	} else {
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
		trade.CloseReason = response.Status
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
	if err := s.logService.LogAction(trade.UserID, "TradeResponse", "Trade status updated", "", metadata); err != nil {
		log.Printf("error: %v", err)
		return nil
	}
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

	accountTypeStr, ok := request["account_type"].(string)
	if !ok {
		return errors.New("missing or invalid account_type")
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

	_, err := s.PlaceTrade(userID, symbol, accountTypeStr, tradeType, orderType, int(leverage), volume, entryPrice, stopLoss, takeProfit, expiration)
	return err
}

func (s *tradeService) HandleBalanceRequest(request map[string]interface{}) error {
	userID, okUser := request["user_id"].(string)
	accountType, ok := request["account_type"].(string)
	if !ok || !okUser {
		return errors.New("missing or invalid user_id or account_type")
	}
	_, err := s.RequestBalance(userID, accountType)
	return err
}

func (s *tradeService) RequestBalance(userID, accountType string) (float64, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return 0, errors.New("invalid user ID")
	}
	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil {
		return 0, errors.New("failed to fetch user")
	}
	if user == nil {
		return 0, errors.New("user not found")
	}
	if !slices.Contains(user.AccountTypes, accountType) {
		return 0, fmt.Errorf("user does not have %s account", accountType)
	}
	balanceRequest := map[string]interface{}{
		"type":         "balance_request",
		"user_id":      userID,
		"account_type": accountType,
		"timestamp":    time.Now().Unix(),
	}

	if err := s.sendToMT5(balanceRequest); err != nil {
		return 0, fmt.Errorf("failed to send balance request: %v", err)
	}

	select {
	case response := <-s.balanceChan:
		if response.UserID != userID || response.AccountType != accountType {
			return 0, errors.New("received balance response for wrong user or account type")
		}
		if response.Error != "" {
			return 0, errors.New(response.Error)
		}
		if response.AccountType == "DEMO" {
			user.DemoMT5Balance = response.Balance
		} else if response.AccountType == "REAL" {
			user.RealMT5Balance = response.Balance
		}
		if err := s.userRepo.UpdateUser(user); err != nil {
			log.Printf("Failed to update user %s balance: %v", accountType, err)
		}
		return response.Balance, nil
	case <-time.After(5 * time.Second):
		if accountType == "DEMO" {
			return user.DemoMT5Balance, nil
		}
		return user.RealMT5Balance, nil
	}
}

func (s *tradeService) CloseTrade(tradeID, userID, accountType string) error {
	tradeObjID, err := primitive.ObjectIDFromHex(tradeID)
	if err != nil {
		return errors.New("invalid trade ID")
	}
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	trade, err := s.tradeRepo.GetTradeByID(tradeObjID)
	if err != nil {
		return err
	}
	if trade == nil {
		return errors.New("trade not found")
	}
	if trade.UserID != userObjID {
		return errors.New("trade belongs to another user")
	}
	if trade.AccountType != accountType {
		return fmt.Errorf("trade is not associated with %s account", accountType)
	}
	if trade.Status != string(models.TradeStatusOpen) {
		return errors.New("trade is not open")
	}
	closeRequest := map[string]interface{}{
		"type":         "close_trade_request",
		"trade_id":     tradeID,
		"user_id":      userID,
		"account_type": accountType,
		"timestamp":    time.Now().Unix(),
	}
	if err := s.sendToMT5(closeRequest); err != nil {
		return fmt.Errorf("failed to send close trade request: %v", err)
	}
	return nil
}

func (s *tradeService) StreamTrades(userID, accountType string) error {
	_, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	if accountType != "DEMO" && accountType != "REAL" {
		return errors.New("invalid account type")
	}
	streamRequest := map[string]interface{}{
		"type":         "order_stream_request",
		"user_id":      userID,
		"account_type": accountType,
		"timestamp":    time.Now().Unix(),
	}
	if err := s.sendToMT5(streamRequest); err != nil {
		return fmt.Errorf("failed to send order stream request: %v", err)
	}
	return nil
}

func (s *tradeService) HandleCloseTradeResponse(response CloseTradeResponse) error {
	if response.Status != "SUCCESS" {
		return fmt.Errorf("MT5 failed to close trade: %s", response.Status)
	}
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
	if trade.AccountType != response.AccountType {
		return fmt.Errorf("trade account type mismatch: expected %s, got %s", trade.AccountType, response.AccountType)
	}
	trade.Status = string(models.TradeStatusClosed)
	trade.CloseTime = &time.Time{}
	secs := int64(response.Timestamp)
	nanos := int64((response.Timestamp - float64(secs)) * 1e9)
	*trade.CloseTime = time.Unix(secs, nanos)
	trade.ClosePrice = response.ClosePrice
	trade.CloseReason = response.CloseReason
	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return err
	}
	user, err := s.userRepo.GetUserByID(trade.UserID)
	if err != nil {
		return err
	}
	if user != nil {
		profit := (response.ClosePrice - trade.EntryPrice) * trade.Volume
		if trade.TradeType == models.TradeTypeSell {
			profit = -profit
		}
		if response.AccountType == "DEMO" {
			user.DemoMT5Balance += profit
		} else if response.AccountType == "REAL" {
			user.RealMT5Balance += profit
		}
		if err := s.userRepo.UpdateUser(user); err != nil {
			log.Printf("Failed to update user balance: %v", err)
		}
	}
	metadata := map[string]interface{}{
		"trade_id":     response.TradeID,
		"account_type": response.AccountType,
		"close_price":  response.ClosePrice,
		"close_reason": response.CloseReason,
	}
	if err := s.logService.LogAction(trade.UserID, "CloseTradeResponse", "Trade closed", "", metadata); err != nil {
		return nil
	}
	s.hub.BroadcastTrade(trade)
	return nil
}

func (s *tradeService) HandleOrderStreamResponse(response OrderStreamResponse) error {
	for _, trade := range response.Trades {
		if trade.AccountType != response.AccountType {
			log.Printf("Trade account type mismatch: trade %s, response %s", trade.AccountType, response.AccountType)
			continue
		}
		existingTrade, err := s.tradeRepo.GetTradeByID(trade.ID)
		if err != nil {
			log.Printf("Failed to check existing trade: %v", err)
			continue
		}
		if existingTrade == nil {
			err = s.tradeRepo.SaveTrade(&trade)
			if err != nil {
				log.Printf("Failed to save streamed trade: %v", err)
				continue
			}
		} else {
			existingTrade.Status = trade.Status
			existingTrade.CloseTime = trade.CloseTime
			existingTrade.ClosePrice = trade.ClosePrice
			existingTrade.CloseReason = trade.CloseReason
			existingTrade.AccountType = trade.AccountType
			err = s.tradeRepo.SaveTrade(existingTrade)
			if err != nil {
				log.Printf("Failed to update streamed trade: %v", err)
				continue
			}
		}
		s.hub.BroadcastTrade(&trade)
	}
	metadata := map[string]interface{}{
		"user_id":      response.UserID,
		"account_type": response.AccountType,
		"trade_count":  len(response.Trades),
	}
	userObjID, _ := primitive.ObjectIDFromHex(response.UserID)
	if err := s.logService.LogAction(userObjID, "OrderStreamResponse", "Order stream processed", "", metadata); err != nil {
		log.Printf("error: %v", err)
		return nil
	}
	return nil
}
