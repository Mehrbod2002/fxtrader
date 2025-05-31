package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/interfaces"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/socket"
	"github.com/mehrbod2002/fxtrader/internal/ws"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	mt5ReconnectBackoffInitial = 2 * time.Second
	mt5ReconnectBackoffMax     = 30 * time.Second
	mt5ReconnectMaxAttempts    = 5
)

type tradeService struct {
	tradeRepo          repository.TradeRepository
	symbolRepo         repository.SymbolRepository
	userRepo           repository.UserRepository
	logService         LogService
	mt5Conn            *websocket.Conn
	mt5ConnMu          sync.Mutex
	responseChan       chan interface{}
	balanceChan        chan interfaces.BalanceResponse
	hub                *ws.Hub
	socketServer       *socket.WebSocketServer
	copyTradeService   CopyTradeService
	tradeResponseChans map[string]chan interfaces.OrderStreamResponse
	tradeResponseMu    sync.Mutex
}

func NewTradeService(tradeRepo repository.TradeRepository, symbolRepo repository.SymbolRepository, userRepo repository.UserRepository, logService LogService, hub *ws.Hub, socketServer *socket.WebSocketServer, copyTradeService CopyTradeService) (interfaces.TradeService, error) {
	return &tradeService{
		tradeRepo:          tradeRepo,
		symbolRepo:         symbolRepo,
		userRepo:           userRepo,
		logService:         logService,
		responseChan:       make(chan interface{}, 100),
		balanceChan:        make(chan interfaces.BalanceResponse, 100),
		hub:                hub,
		socketServer:       socketServer,
		copyTradeService:   copyTradeService,
		tradeResponseChans: make(map[string]chan interfaces.OrderStreamResponse),
	}, nil
}

func (s *tradeService) RegisterMT5Connection(conn *websocket.Conn) {
	s.mt5ConnMu.Lock()
	s.mt5Conn = conn
	s.mt5ConnMu.Unlock()
	log.Printf("Registered MT5 connection")
}

func (s *tradeService) sendToMT5(msg interface{}) error {
	switch msg.(type) {
	case map[string]interface{}:
		message := msg.(map[string]interface{})
		msgType, ok := message["type"].(string)
		if !ok {
			return fmt.Errorf("missing or invalid message type")
		}
		switch msgType {
		case "trade_request":
			return s.socketServer.SendTradeRequest(message)
		case "close_trade_request":
			return s.socketServer.SendCloseTradeRequest(message)
		case "order_stream_request":
			return s.socketServer.SendOrderStreamRequest(message)
		case "balance_request":
			return s.socketServer.SendBalanceRequest(message)
		case "modify_trade_request":
			return s.socketServer.SendTradeRequest(message)
		default:
			return fmt.Errorf("unsupported message type: %s", msgType)
		}
	default:
		return fmt.Errorf("invalid message format")
	}
}

func (s *tradeService) PlaceTrade(userID, symbol, accountType string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, interfaces.TradeResponse, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, interfaces.TradeResponse{}, errors.New("invalid user ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil {
		return nil, interfaces.TradeResponse{}, errors.New("failed to fetch user")
	}
	if user == nil {
		return nil, interfaces.TradeResponse{}, errors.New("user not found")
	}

	if !slices.Contains(user.AccountTypes, accountType) {
		return nil, interfaces.TradeResponse{}, fmt.Errorf("user does not have %s account", accountType)
	}

	symbols, err := s.symbolRepo.GetAllSymbols()
	if err != nil {
		return nil, interfaces.TradeResponse{}, errors.New("failed to fetch symbols")
	}

	var symbolObj *models.Symbol
	for _, sym := range symbols {
		if sym.SymbolName == symbol {
			symbolObj = sym
			break
		}
	}
	if symbolObj == nil {
		return nil, interfaces.TradeResponse{}, errors.New("symbol not found")
	}

	requiredMargin := volume * entryPrice / float64(leverage)
	if user.Balance < requiredMargin+symbolObj.CommissionFee {
		return nil, interfaces.TradeResponse{}, errors.New("insufficient balance")
	}

	if tradeType != models.TradeTypeBuy && tradeType != models.TradeTypeSell {
		return nil, interfaces.TradeResponse{}, errors.New("invalid trade type")
	}

	validOrderTypes := []string{"MARKET", "BUY_STOP", "SELL_STOP", "BUY_LIMIT", "SELL_LIMIT"}
	isValidOrderType := slices.Contains(validOrderTypes, orderType)
	if !isValidOrderType {
		return nil, interfaces.TradeResponse{}, errors.New("invalid order type")
	}

	if volume < symbolObj.MinLot || volume > symbolObj.MaxLot {
		return nil, interfaces.TradeResponse{}, errors.New("volume out of allowed range")
	}

	if leverage > symbolObj.Leverage {
		return nil, interfaces.TradeResponse{}, errors.New("leverage exceeds symbol limit")
	}

	if orderType != "MARKET" && entryPrice <= 0 {
		return nil, interfaces.TradeResponse{}, errors.New("entry price required for non-market orders")
	}
	if orderType == "MARKET" && entryPrice > 0 {
		return nil, interfaces.TradeResponse{}, errors.New("entry price not allowed for market orders")
	}

	if stopLoss < 0 || takeProfit < 0 {
		return nil, interfaces.TradeResponse{}, errors.New("stop loss and take profit cannot be negative")
	}

	if expiration != nil && expiration.Before(time.Now()) {
		return nil, interfaces.TradeResponse{}, errors.New("expiration time must be in the future")
	}

	user.Balance -= requiredMargin + symbolObj.CommissionFee
	if err := s.userRepo.UpdateUser(user); err != nil {
		return nil, interfaces.TradeResponse{}, fmt.Errorf("failed to update user balance: %v", err)
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

	responseChan := make(chan interfaces.OrderStreamResponse, 1)
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
		user.Balance += requiredMargin
		s.userRepo.UpdateUser(user)
		return nil, interfaces.TradeResponse{}, err
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		user.Balance += requiredMargin
		s.userRepo.UpdateUser(user)
		return nil, interfaces.TradeResponse{}, err
	}

	var tradeResponse interfaces.TradeResponse
	select {
	case response := <-responseChan:
		tr, ok := response.(interfaces.TradeResponse)
		if !ok {
			user.Balance += requiredMargin
			return nil, interfaces.TradeResponse{}, errors.New("invalid trade response type")
		}
		tradeResponse = tr
		if tradeResponse.TradeID != trade.ID.Hex() {
			user.Balance += requiredMargin
			s.userRepo.UpdateUser(user)
			return nil, interfaces.TradeResponse{}, errors.New("received response for wrong trade ID")
		}
		trade.Status = tradeResponse.Status
		trade.MatchedTradeID = tradeResponse.MatchedTradeID
		if tradeResponse.Status == "MATCHED" {
			trade.Status = string(models.TradeStatusOpen)
		} else if tradeResponse.Status == "PENDING" {
			trade.Status = string(models.TradeStatusPending)
		} else {
			trade.Status = string(models.TradeStatusClosed)
			trade.CloseTime = &time.Time{}
			*trade.CloseTime = time.Now()
			trade.CloseReason = tradeResponse.Status
			err = s.tradeRepo.SaveTrade(trade)
			user.Balance += requiredMargin
			s.userRepo.UpdateUser(user)
			return nil, interfaces.TradeResponse{}, fmt.Errorf("trade failed with status: %s", tradeResponse.Status)
		}
		err = s.tradeRepo.SaveTrade(trade)
		if err != nil {
			user.Balance += requiredMargin
			s.userRepo.UpdateUser(user)
			return nil, interfaces.TradeResponse{}, err
		}
	case <-time.After(30 * time.Second):
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
		trade.CloseReason = "TIMEOUT"
		err = s.tradeRepo.SaveTrade(trade)
		user.Balance += requiredMargin
		s.userRepo.UpdateUser(user)
		return nil, interfaces.TradeResponse{}, errors.New("timeout waiting for MT5 trade response")
	}

	go func() {
		if err := s.copyTradeService.MirrorTrade(trade, accountType); err != nil {
			log.Printf("Failed to mirror trade: %v", err)
		}
	}()

	return trade, tradeResponse, nil
}

func (s *tradeService) HandleBalanceResponse(response interfaces.BalanceResponse) error {
	userObjID, err := primitive.ObjectIDFromHex(response.UserID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %v", err)
	}
	if user == nil {
		return errors.New("user not found")
	}
	if !slices.Contains(user.AccountTypes, response.AccountType) {
		return fmt.Errorf("user does not have %s account", response.AccountType)
	}

	balanceData := &models.BalanceData{
		UserID:      response.UserID,
		AccountType: response.AccountType,
		Balance:     response.Balance,
		Timestamp:   time.Now().Unix(),
	}
	s.hub.BroadcastBalance(balanceData)

	return nil
}

func (s *tradeService) HandleTradeResponse(response interfaces.TradeResponse) error {
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

	if response.MatchedVolume > 0 {
		trade.Volume -= response.MatchedVolume
	}

	if response.Status == "MATCHED" {
		trade.Status = string(models.TradeStatusOpen)
		trade.MatchedTradeID = response.MatchedTradeID
	} else if response.Status == "PENDING" {
		trade.Status = string(models.TradeStatusPending)
	} else {
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
		trade.CloseReason = response.Status
		user, err := s.userRepo.GetUserByID(trade.UserID)
		if err == nil && user != nil {
			margin := trade.Volume * trade.EntryPrice / float64(trade.Leverage)
			user.Balance += margin
			s.userRepo.UpdateUser(user)
		}
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
	}

	s.tradeResponseMu.Lock()
	if ch, exists := s.tradeResponseChans[response.TradeID]; exists {
		select {
		case ch <- response:
		default:
			log.Printf("Trade response channel for trade %s is full or closed", response.TradeID)
		}
	}
	s.tradeResponseMu.Unlock()

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

	_, _, err := s.PlaceTrade(userID, symbol, accountTypeStr, tradeType, orderType, int(leverage), volume, entryPrice, stopLoss, takeProfit, expiration)
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
	return user.Balance, nil
}

func (s *tradeService) CloseTrade(tradeID, userID, accountType string) (interfaces.CloseTradeResponse, error) {
	tradeObjID, err := primitive.ObjectIDFromHex(tradeID)
	if err != nil {
		return interfaces.CloseTradeResponse{}, errors.New("invalid trade ID")
	}
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return interfaces.CloseTradeResponse{}, errors.New("invalid user ID")
	}
	trade, err := s.tradeRepo.GetTradeByID(tradeObjID)
	if err != nil {
		return interfaces.CloseTradeResponse{}, err
	}
	if trade == nil {
		return interfaces.CloseTradeResponse{}, errors.New("trade not found")
	}
	if trade.UserID != userObjID {
		return interfaces.CloseTradeResponse{}, errors.New("trade belongs to another user")
	}
	if trade.AccountType != accountType {
		return interfaces.CloseTradeResponse{}, fmt.Errorf("trade is not associated with %s account", accountType)
	}
	if trade.Status != string(models.TradeStatusOpen) {
		return interfaces.CloseTradeResponse{}, errors.New("trade is not open")
	}

	closeRequest := map[string]interface{}{
		"type":         "close_trade_request",
		"trade_id":     tradeID,
		"user_id":      userID,
		"account_type": accountType,
		"timestamp":    time.Now().Unix(),
	}

	responseChan := make(chan interfaces.OrderStreamResponse, 1)
	s.tradeResponseMu.Lock()
	s.tradeResponseChans[tradeID] = responseChan
	s.tradeResponseMu.Unlock()

	defer func() {
		s.tradeResponseMu.Lock()
		delete(s.tradeResponseChans, tradeID)
		close(responseChan)
		s.tradeResponseMu.Unlock()
	}()

	if err := s.sendToMT5(closeRequest); err != nil {
		return interfaces.CloseTradeResponse{}, fmt.Errorf("failed to send close trade request: %v", err)
	}

	select {
	case response := <-responseChan:
		closeResponse, ok := response.(interfaces.CloseTradeResponse)
		if !ok {
			return interfaces.CloseTradeResponse{}, errors.New("invalid close trade response type")
		}
		if closeResponse.TradeID != tradeID {
			return interfaces.CloseTradeResponse{}, errors.New("received response for wrong trade ID")
		}
		return closeResponse, nil
	case <-time.After(30 * time.Second):
		return interfaces.CloseTradeResponse{}, errors.New("timeout waiting for MT5 close trade response")
	}
}

func (s *tradeService) StreamTrades(userID, accountType string) (chan interfaces.OrderStreamResponse, error) {
	_, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}
	if accountType != "DEMO" && accountType != "REAL" {
		return nil, errors.New("invalid account type")
	}

	streamKey := userID + ":" + accountType
	tradeChan := make(chan interface{}, 100)

	s.tradeResponseMu.Lock()
	s.tradeResponseChans[streamKey] = tradeChan
	s.tradeResponseMu.Unlock()

	streamRequest := map[string]interface{}{
		"type":         "order_stream_request",
		"user_id":      userID,
		"account_type": accountType,
		"timestamp":    time.Now().Unix(),
	}

	if err := s.sendToMT5(streamRequest); err != nil {
		s.tradeResponseMu.Lock()
		delete(s.tradeResponseChans, streamKey)
		close(tradeChan)
		s.tradeResponseMu.Unlock()
		return nil, fmt.Errorf("failed to send order stream request: %v", err)
	}

	go func() {
		<-time.After(24 * time.Hour)
		s.tradeResponseMu.Lock()
		delete(s.tradeResponseChans, streamKey)
		close(tradeChan)
		s.tradeResponseMu.Unlock()
	}()

	return tradeChan, nil
}

func (s *tradeService) HandleCloseTradeResponse(response interfaces.CloseTradeResponse) error {
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

	user, err := s.userRepo.GetUserByID(trade.UserID)
	if err != nil {
		return err
	}
	if user != nil {
		profit := (response.ClosePrice - trade.EntryPrice) * trade.Volume
		if trade.TradeType == models.TradeTypeSell {
			profit = -profit
		}

		margin := trade.Volume * trade.EntryPrice / float64(trade.Leverage)
		user.Balance += profit + margin
		if err := s.userRepo.UpdateUser(user); err != nil {
			log.Printf("Failed to update user balance: %v", err)
		}
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"trade_id":     response.TradeID,
		"account_type": response.AccountType,
		"close_price":  response.ClosePrice,
		"close_reason": response.CloseReason,
	}
	if err := s.logService.LogAction(trade.UserID, "CloseTradeResponse", "Trade closed", "", metadata); err != nil {
		log.Printf("error: %v", err)
	}

	s.tradeResponseMu.Lock()
	if ch, exists := s.tradeResponseChans[response.TradeID]; exists {
		select {
		case ch <- response:
		default:
			log.Printf("Close trade response channel for trade %s is full or closed", response.TradeID)
		}
	}
	s.tradeResponseMu.Unlock()

	s.hub.BroadcastTrade(trade)
	return nil
}

func (s *tradeService) HandleOrderStreamResponse(response interfaces.OrderStreamResponse) error {
	userObjID, err := primitive.ObjectIDFromHex(response.UserID)
	if err != nil {
		return errors.New("invalid user ID")
	}

	for _, trade := range response.Trades {
		if trade.UserID != userObjID || trade.AccountType != response.AccountType {
			log.Printf("Trade user or account mismatch: trade %s, response %s", trade.UserID.Hex(), response.UserID)
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
			existingTrade.MatchedTradeID = trade.MatchedTradeID
			existingTrade.Volume = trade.Volume
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
	if err := s.logService.LogAction(userObjID, "OrderStreamResponse", "Order stream processed", "", metadata); err != nil {
		log.Printf("error: %v", err)
	}

	streamKey := response.UserID + response.AccountType
	s.tradeResponseMu.Lock()
	if ch, exists := s.tradeResponseChans[streamKey]; exists {
		select {
		case ch <- response:
		default:
			log.Printf("Order stream response channel for user %s is full", response.UserID)
		}
	}
	s.tradeResponseMu.Unlock()

	return nil
}

func (s *tradeService) ModifyTrade(ctx context.Context, userID, tradeID, accountType string, entryPrice, volume float64) (interfaces.TradeResponse, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid user ID")
	}
	tradeObjID, err := primitive.ObjectIDFromHex(tradeID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid trade ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil || user == nil {
		return interfaces.TradeResponse{}, errors.New("user not found")
	}

	trade, err := s.tradeRepo.GetTradeByID(tradeObjID)
	if err != nil {
		return interfaces.TradeResponse{}, err
	}
	if trade == nil {
		return interfaces.TradeResponse{}, errors.New("trade not found")
	}
	if trade.UserID != userObjID || trade.AccountType != accountType {
		return interfaces.TradeResponse{}, errors.New("trade does not belong to user or account")
	}
	if trade.Status != string(models.TradeStatusPending) {
		return interfaces.TradeResponse{}, errors.New("only pending trades can be modified")
	}

	if entryPrice <= 0 && volume <= 0 {
		return interfaces.TradeResponse{}, errors.New("at least one of entry price or volume must be provided")
	}
	if volume > 0 {
		if volume < 0.01 || volume > 100 {
			return interfaces.TradeResponse{}, errors.New("invalid volume")
		}
	}

	request := map[string]interface{}{
		"type":         "modify_trade_request",
		"trade_id":     tradeID,
		"user_id":      userID,
		"account_type": accountType,
		"entry_price":  entryPrice,
		"volume":       volume,
	}

	responseChan := make(chan interfaces.OrderStreamResponse, 1)
	s.tradeResponseMu.Lock()
	s.tradeResponseChans[tradeID] = responseChan
	s.tradeResponseMu.Unlock()
	defer func() {
		s.tradeResponseMu.Lock()
		delete(s.tradeResponseChans, tradeID)
		close(responseChan)
		s.tradeResponseMu.Unlock()
	}()

	if err := s.sendToMT5(request); err != nil {
		return interfaces.TradeResponse{}, fmt.Errorf("failed to send modify request: %v", err)
	}

	select {
	case response := <-responseChan:
		resp, ok := response.(interfaces.TradeResponse)
		if !ok {
			return interfaces.TradeResponse{}, errors.New("invalid response type")
		}
		if resp.Status == "MODIFIED" {
			if entryPrice > 0 {
				trade.EntryPrice = entryPrice
			}
			if volume > 0 {
				trade.Volume = volume
			}
			if err := s.tradeRepo.SaveTrade(trade); err != nil {
				log.Printf("Failed to save modified trade: %v", err)
			}
			s.logService.LogAction(userObjID, "ModifyTrade", fmt.Sprintf("Modified trade %s: entry_price=%f, volume=%f", tradeID, entryPrice, volume), "", nil)
		}
		return resp, nil
	case <-time.After(10 * time.Second):
		return interfaces.TradeResponse{}, errors.New("timeout waiting for modify response")
	}
}
