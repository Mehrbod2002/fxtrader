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
	"github.com/mehrbod2002/fxtrader/internal/constants"
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
	tradeRepo           repository.TradeRepository
	symbolRepo          repository.SymbolRepository
	userRepo            repository.UserRepository
	accountRepo         repository.AccountRepository
	logService          LogService
	mt5Conn             *websocket.Conn
	mt5ConnMu           sync.Mutex
	responseChan        chan interface{}
	balanceChan         chan interfaces.BalanceResponse
	hub                 *ws.Hub
	socketServer        *socket.WebSocketServer
	copyTradeService    CopyTradeService
	tradeResponseChans  map[string]chan interfaces.TradeResponse
	tradeResponseMu     sync.Mutex
	streamCtx           map[string]context.CancelFunc
	ordersResponseChans map[string]chan models.OrderStreamResponse
	ordersResponseMu    sync.Mutex
}

func NewTradeService(
	tradeRepo repository.TradeRepository,
	symbolRepo repository.SymbolRepository,
	userRepo repository.UserRepository,
	accountRepo repository.AccountRepository,
	logService LogService,
	hub *ws.Hub,
	socketServer *socket.WebSocketServer,
	copyTradeService CopyTradeService,
) (interfaces.TradeService, error) {
	return &tradeService{
		tradeRepo:           tradeRepo,
		symbolRepo:          symbolRepo,
		userRepo:            userRepo,
		accountRepo:         accountRepo,
		logService:          logService,
		responseChan:        make(chan interface{}, 100),
		balanceChan:         make(chan interfaces.BalanceResponse, 100),
		hub:                 hub,
		socketServer:        socketServer,
		copyTradeService:    copyTradeService,
		tradeResponseChans:  make(map[string]chan interfaces.TradeResponse),
		streamCtx:           make(map[string]context.CancelFunc),
		ordersResponseChans: make(map[string]chan models.OrderStreamResponse),
	}, nil
}

func (s *tradeService) RegisterMT5Connection(conn *websocket.Conn) {
	s.mt5ConnMu.Lock()
	s.mt5Conn = conn
	s.mt5ConnMu.Unlock()
}

func (s *tradeService) RegisterWallet(userID, accountID, walletID string) error {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	accountObjID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return errors.New("invalid account ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	account, err := s.accountRepo.GetAccountByID(accountObjID)
	if err != nil || account == nil {
		return errors.New("account not found")
	}
	if account.UserID != userObjID {
		return errors.New("account does not belong to user")
	}

	// Validate wallet ID (example: ensure it's not empty and follows a specific format)
	if walletID == "" || len(walletID) < 8 {
		return errors.New("invalid wallet ID")
	}

	account.WalletID = walletID
	if err := s.accountRepo.UpdateAccount(account); err != nil {
		return fmt.Errorf("failed to update account with wallet ID: %v", err)
	}

	return nil
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

func (s *tradeService) PlaceTrade(userID, accountID, symbol, accountType string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, interfaces.TradeResponse, error) {
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

	account, err := s.accountRepo.GetAccountByName(accountID, userObjID)
	if err != nil {
		return nil, interfaces.TradeResponse{}, errors.New("failed to fetch account")
	}
	if account == nil || account.UserID != userObjID {
		return nil, interfaces.TradeResponse{}, errors.New("account not found or does not belong to user")
	}
	if account.AccountType != accountType {
		return nil, interfaces.TradeResponse{}, fmt.Errorf("account type mismatch: expected %s, got %s", account.AccountType, accountType)
	}

	symbols, err := s.symbolRepo.GetAllSymbols()
	if err != nil {
		return nil, interfaces.TradeResponse{}, errors.New("failed to fetch symbols")
	}

	var symbolObj *models.Symbol
	for _, sym := range symbols {
		if sym.DisplayName == symbol {
			symbolObj = sym
			symbol = sym.SymbolName
			break
		}
	}
	if symbolObj == nil {
		return nil, interfaces.TradeResponse{}, errors.New("symbol not found")
	}

	requiredMargin := volume * entryPrice / float64(leverage)
	if account.Balance < requiredMargin+symbolObj.CommissionFee {
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

	account.Balance -= requiredMargin + symbolObj.CommissionFee
	if err := s.accountRepo.UpdateAccount(account); err != nil {
		return nil, interfaces.TradeResponse{}, fmt.Errorf("failed to update account balance: %v", err)
	}

	trade := &models.TradeHistory{
		ID:          primitive.NewObjectID(),
		UserID:      userObjID,
		AccountID:   account.ID,
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
		"trade_code":   "",
		"user_id":      trade.UserID.Hex(),
		"account_id":   trade.AccountID.Hex(),
		"account_type": accountType,
		"account_name": accountID,
		"wallet_id":    account.WalletID,
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

	responseChan := make(chan interfaces.TradeResponse, 1)
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
		account.Balance += requiredMargin + symbolObj.CommissionFee
		s.accountRepo.UpdateAccount(account)
		return nil, interfaces.TradeResponse{}, err
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		account.Balance += requiredMargin + symbolObj.CommissionFee
		s.accountRepo.UpdateAccount(account)
		return nil, interfaces.TradeResponse{}, err
	}

	var tradeResponse interfaces.TradeResponse
	select {
	case response := <-responseChan:
		tradeResponse = response
		if tradeResponse.TradeID != trade.ID.Hex() {
			account.Balance += requiredMargin + symbolObj.CommissionFee
			s.accountRepo.UpdateAccount(account)
			return nil, interfaces.TradeResponse{}, errors.New("received response for wrong trade ID")
		}
		trade.Status = tradeResponse.Status
		trade.MatchedTradeID = tradeResponse.MatchedTradeID

		switch tradeResponse.Status {
		case "MATCHED":
			trade.Status = string(models.TradeStatusOpen)
		case "PENDING":
			trade.Status = string(models.TradeStatusPending)
		default:
			trade.Status = string(models.TradeStatusClosed)
			trade.CloseTime = &time.Time{}
			*trade.CloseTime = time.Now()
			trade.CloseReason = tradeResponse.Status
			_ = s.tradeRepo.SaveTrade(trade)
			account.Balance += requiredMargin + symbolObj.CommissionFee
			s.accountRepo.UpdateAccount(account)
			return nil, interfaces.TradeResponse{}, fmt.Errorf("%s", constants.TradeRetcodes[tradeResponse.TradeRetcode]["fa"])
		}

		err = s.tradeRepo.SaveTrade(trade)
		if err != nil {
			account.Balance += requiredMargin + symbolObj.CommissionFee
			s.accountRepo.UpdateAccount(account)
			return nil, interfaces.TradeResponse{}, err
		}
	case <-time.After(30 * time.Second):
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
		trade.CloseReason = "TIMEOUT"
		_ = s.tradeRepo.SaveTrade(trade)
		account.Balance += requiredMargin + symbolObj.CommissionFee
		s.accountRepo.UpdateAccount(account)
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
	accountObjID, err := primitive.ObjectIDFromHex(response.AccountID)
	if err != nil {
		return errors.New("invalid account ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %v", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	account, err := s.accountRepo.GetAccountByID(accountObjID)
	if err != nil {
		return fmt.Errorf("failed to fetch account: %v", err)
	}
	if account == nil || account.UserID != userObjID {
		return errors.New("account not found or does not belong to user")
	}
	if account.AccountType != response.AccountType {
		return fmt.Errorf("account type mismatch: expected %s, got %s", account.AccountType, response.AccountType)
	}

	account.Balance = response.Balance
	if err := s.accountRepo.UpdateAccount(account); err != nil {
		return fmt.Errorf("failed to update account balance: %v", err)
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

	account, err := s.accountRepo.GetAccountByID(trade.AccountID)
	if err != nil || account == nil {
		return errors.New("account not found")
	}

	if response.MatchedVolume > 0 {
		trade.Volume -= response.MatchedVolume
	}

	switch response.Status {
	case "MATCHED":
		trade.Status = string(models.TradeStatusOpen)
		trade.MatchedTradeID = response.MatchedTradeID
	case "PENDING":
		trade.Status = string(models.TradeStatusPending)
	default:
		trade.Status = string(models.TradeStatusClosed)
		trade.CloseTime = &time.Time{}
		*trade.CloseTime = time.Now()
		trade.CloseReason = response.Status
		margin := trade.Volume * trade.EntryPrice / float64(trade.Leverage)
		account.Balance += margin
		s.accountRepo.UpdateAccount(account)
	}
	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"trade_id":         response.TradeID,
		"account_id":       trade.AccountID.Hex(),
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

	accountID, ok := request["account_id"].(string)
	if !ok {
		return errors.New("missing or invalid account_id")
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

	walletID, ok := request["wallet_id"].(string)
	if !ok || walletID == "" {
		return errors.New("missing or invalid wallet_id")
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

	accountObjID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return errors.New("invalid account ID")
	}
	account, err := s.accountRepo.GetAccountByID(accountObjID)
	if err != nil || account == nil {
		return errors.New("account not found")
	}
	if account.WalletID != walletID {
		return errors.New("wallet ID mismatch")
	}

	_, _, err = s.PlaceTrade(userID, accountID, symbol, accountTypeStr, tradeType, orderType, int(leverage), volume, entryPrice, stopLoss, takeProfit, expiration)
	return err
}

func (s *tradeService) HandleBalanceRequest(request map[string]interface{}) error {
	userID, ok := request["user_id"].(string)
	if !ok {
		return errors.New("missing or invalid user_id")
	}
	accountID, ok := request["account_id"].(string)
	if !ok {
		return errors.New("missing or invalid account_id")
	}
	accountType, ok := request["account_type"].(string)
	if !ok {
		return errors.New("missing or invalid account_type")
	}
	_, err := s.RequestBalance(userID, accountID, accountType)
	return err
}

func (s *tradeService) RequestBalance(userID, accountID, accountType string) (float64, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return 0, errors.New("invalid user ID")
	}

	accountObjID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return 0, errors.New("invalid account ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil {
		return 0, errors.New("failed to fetch user")
	}

	if user == nil {
		return 0, errors.New("user not found")
	}

	account, err := s.accountRepo.GetAccountByID(accountObjID)
	if err != nil {
		return 0, errors.New("failed to fetch account")
	}

	if account == nil || account.UserID != userObjID {
		return 0, errors.New("account not found or does not belong to user")
	}

	if account.AccountType != accountType {
		return 0, fmt.Errorf("account type mismatch: expected %s, got %s", account.AccountType, accountType)
	}

	balanceRequest := map[string]interface{}{
		"type":         "balance_request",
		"account_name": accountObjID,
		"user_id":      userID,
		"account_id":   accountID,
		"account_type": accountType,
		"wallet_id":    account.WalletID,
		"timestamp":    time.Now().Unix(),
	}

	if err := s.sendToMT5(balanceRequest); err != nil {
		return 0, fmt.Errorf("failed to send balance request: %v", err)
	}

	select {
	case response := <-s.balanceChan:
		if response.UserID != userID || response.AccountID != accountID || response.AccountType != accountType {
			return 0, errors.New("invalid balance response")
		}
		return response.Balance, nil
	case <-time.After(10 * time.Second):
		return 0, errors.New("timeout waiting for balance response")
	}
}

func (s *tradeService) CloseTrade(tradeID, userID, accountType, accountID string) (interfaces.TradeResponse, error) {
	tradeObjID, err := primitive.ObjectIDFromHex(tradeID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid trade ID")
	}
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid user ID")
	}
	accountObjID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid account ID")
	}
	trade, err := s.tradeRepo.GetTradeByID(tradeObjID)
	if err != nil {
		return interfaces.TradeResponse{}, err
	}
	if trade == nil {
		return interfaces.TradeResponse{}, errors.New("trade not found")
	}
	if trade.UserID != userObjID || trade.AccountID != accountObjID {
		return interfaces.TradeResponse{}, errors.New("trade belongs to another user or account")
	}
	if trade.AccountType != accountType {
		return interfaces.TradeResponse{}, fmt.Errorf("trade is not associated with %s account", accountType)
	}

	account, err := s.accountRepo.GetAccountByID(accountObjID)
	if err != nil || account == nil {
		return interfaces.TradeResponse{}, errors.New("account not found")
	}

	closeRequest := map[string]interface{}{
		"type":         "close_trade_request",
		"trade_id":     tradeID,
		"user_id":      userID,
		"account_id":   accountID,
		"account_type": accountType,
		"wallet_id":    account.WalletID, // Include wallet ID
		"timestamp":    time.Now().Unix(),
	}

	responseChan := make(chan interfaces.TradeResponse, 1)
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
		return interfaces.TradeResponse{}, fmt.Errorf("failed to send close trade request: %v", err)
	}

	select {
	case response := <-responseChan:
		if response.TradeID != tradeID {
			return interfaces.TradeResponse{}, errors.New("received response for wrong trade ID")
		}
		return response, nil
	case <-time.After(30 * time.Second):
		return interfaces.TradeResponse{}, errors.New("timeout waiting for MT5 close trade response")
	}
}

func (s *tradeService) StreamTrades(userID, accountType string) (chan models.OrderStreamResponse, error) {
	if accountType != "DEMO" && accountType != "REAL" {
		return nil, errors.New("invalid account type")
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}
	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	streamKey := userID + ":" + accountType
	ctx, cancel := context.WithCancel(context.Background())
	streamChan := make(chan models.OrderStreamResponse, 256)

	s.ordersResponseMu.Lock()
	s.streamCtx[streamKey] = cancel
	s.ordersResponseChans[streamKey] = streamChan
	s.ordersResponseMu.Unlock()

	streamRequest := map[string]interface{}{
		"type":         "order_stream_request",
		"user_id":      userID,
		"account_type": accountType,
		"timestamp":    time.Now().Unix(),
	}

	if err := s.sendToMT5(streamRequest); err != nil {
		s.ordersResponseMu.Lock()
		delete(s.streamCtx, streamKey)
		delete(s.ordersResponseChans, streamKey)
		s.ordersResponseMu.Unlock()
		close(streamChan)
		return nil, fmt.Errorf("failed to send order stream request: %v", err)
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-time.After(24 * time.Hour):
		}
		s.ordersResponseMu.Lock()
		if cancel, exists := s.streamCtx[streamKey]; exists {
			cancel()
			delete(s.streamCtx, streamKey)
			delete(s.ordersResponseChans, streamKey)
		}
		s.ordersResponseMu.Unlock()
		close(streamChan)
	}()

	return streamChan, nil
}

func (s *tradeService) StopStream(userID, accountType string) error {
	streamKey := userID + ":" + accountType
	s.tradeResponseMu.Lock()
	defer s.tradeResponseMu.Unlock()

	if cancel, exists := s.streamCtx[streamKey]; exists {
		cancel()
		delete(s.streamCtx, streamKey)
		return nil
	}
	return fmt.Errorf("no active stream found for user %s and account type %s", userID, accountType)
}

func (s *tradeService) HandleCloseTradeResponse(response interfaces.TradeResponse) error {
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

	account, err := s.accountRepo.GetAccountByID(trade.AccountID)
	if err != nil || account == nil {
		return errors.New("account not found")
	}

	trade.Status = string(models.TradeStatusClosed)
	trade.CloseTime = &time.Time{}
	secs := int64(response.Timestamp)
	nanos := int64((response.Timestamp - float64(secs)) * 1e9)
	*trade.CloseTime = time.Unix(secs, nanos)
	trade.ClosePrice = response.ClosePrice
	trade.CloseReason = response.CloseReason

	profit := (response.ClosePrice - trade.EntryPrice) * trade.Volume
	if trade.TradeType == models.TradeTypeSell {
		profit = -profit
	}

	margin := trade.Volume * trade.EntryPrice / float64(trade.Leverage)
	account.Balance += profit + margin
	if err := s.accountRepo.UpdateAccount(account); err != nil {
		log.Printf("Failed to update account balance: %v", err)
	}

	err = s.tradeRepo.SaveTrade(trade)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"trade_id":     response.TradeID,
		"account_id":   trade.AccountID.Hex(),
		"account_type": response.AccountType,
		"close_price":  response.ClosePrice,
		"close_reason": response.CloseReason,
	}
	if err := s.logService.LogAction(trade.UserID, "TradeResponse", "Trade closed", "", metadata); err != nil {
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

func (s *tradeService) HandleOrderStreamResponse(response models.OrderStreamResponse) error {
	for _, trade := range response.Trades {
		if trade.AccountType != response.AccountType {
			continue
		}

		existingTrade, err := s.tradeRepo.GetTradeByID(trade.ID)
		if err != nil {
			continue
		}

		openTime := time.Unix(trade.OpenTime, 0)
		trade := models.TradeHistory{
			ID:             trade.ID,
			UserID:         response.UserID,
			AccountID:      trade.AccountID,
			Symbol:         trade.Symbol,
			TradeType:      models.TradeType(trade.TradeType),
			OrderType:      trade.OrderType,
			Leverage:       0,
			Volume:         trade.Volume,
			EntryPrice:     trade.EntryPrice,
			ClosePrice:     0,
			StopLoss:       trade.StopLoss,
			TakeProfit:     trade.TakeProfit,
			OpenTime:       openTime,
			CloseTime:      nil,
			CloseReason:    "",
			Status:         trade.Status,
			MatchedTradeID: "",
			Expiration:     nil,
			AccountType:    trade.AccountType,
		}

		if existingTrade == nil {
			if err = s.tradeRepo.SaveTrade(&trade); err != nil {
				continue
			}
		} else {
			existingTrade.Status = trade.Status
			existingTrade.AccountType = trade.AccountType
			existingTrade.AccountID = trade.AccountID
			existingTrade.Volume = trade.Volume
			if err = s.tradeRepo.SaveTrade(existingTrade); err != nil {
				continue
			}
		}
		s.hub.BroadcastTrade(&trade)
	}

	metadata := map[string]interface{}{
		"user_id":      response.UserID.Hex(),
		"account_type": response.AccountType,
		"trade_count":  len(response.Trades),
	}
	if err := s.logService.LogAction(response.UserID, "OrderStreamResponse", "Order stream processed", "", metadata); err != nil {
		log.Printf("Failed to log order stream action: %v", err)
	}

	s.ordersResponseMu.Lock()
	streamKey := response.UserID.Hex() + ":" + response.AccountType
	if ch, exists := s.ordersResponseChans[streamKey]; exists {
		select {
		case ch <- response:
		default:
			log.Printf("Stream channel for %s is full or closed", streamKey)
		}
	}
	s.ordersResponseMu.Unlock()

	s.hub.BroadcastOrderStream(response)

	return nil
}

func (s *tradeService) ModifyTrade(ctx context.Context, userID, tradeID, accountType, accountID string, entryPrice, volume float64) (interfaces.TradeResponse, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid user ID")
	}
	tradeObjID, err := primitive.ObjectIDFromHex(tradeID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid trade ID")
	}
	accountObjID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return interfaces.TradeResponse{}, errors.New("invalid account ID")
	}

	user, err := s.userRepo.GetUserByID(userObjID)
	if err != nil || user == nil {
		return interfaces.TradeResponse{}, errors.New("user not found")
	}

	account, err := s.accountRepo.GetAccountByID(accountObjID)
	if err != nil || account == nil || account.UserID != userObjID {
		return interfaces.TradeResponse{}, errors.New("account not found or does not belong to user")
	}
	if account.AccountType != accountType {
		return interfaces.TradeResponse{}, fmt.Errorf("account type mismatch: expected %s, got %s", account.AccountType, accountType)
	}

	trade, err := s.tradeRepo.GetTradeByID(tradeObjID)
	if err != nil {
		return interfaces.TradeResponse{}, err
	}
	if trade == nil {
		return interfaces.TradeResponse{}, errors.New("trade not found")
	}

	if trade.UserID != userObjID || trade.AccountID != accountObjID {
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
		"account_id":   accountID,
		"account_type": accountType,
		"wallet_id":    account.WalletID, // Include wallet ID
		"entry_price":  entryPrice,
		"volume":       volume,
	}

	responseChan := make(chan interfaces.TradeResponse, 1)
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
		if response.Status == "MODIFIED" {
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
		return response, nil
	case <-time.After(10 * time.Second):
		return interfaces.TradeResponse{}, errors.New("timeout waiting for modify response")
	}
}
