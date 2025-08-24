package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mehrbod2002/fxtrader/interfaces"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/service"
	"github.com/mehrbod2002/fxtrader/internal/ws"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeHandler struct {
	tradeService interfaces.TradeService
	logService   service.LogService
	hub          *ws.Hub
}

func NewTradeHandler(tradeService interfaces.TradeService, logService service.LogService, hub *ws.Hub) *TradeHandler {
	return &TradeHandler{
		tradeService: tradeService,
		logService:   logService,
		hub:          hub,
	}
}

// @Summary Register a wallet for trading
// @Description Allows an authenticated user to register a wallet for a specific account
// @Tags Trades
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wallet body WalletRequest true "Wallet details"
// @Success 200 {object} map[string]string "Wallet registered"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Invalid account"
// @Failure 500 {object} map[string]string "Server error"
// @Router /wallets/register [post]
func (h *TradeHandler) RegisterWallet(c *gin.Context) {
	var req WalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID := c.GetString("user_id")
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	_, err = primitive.ObjectIDFromHex(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	if err := h.tradeService.RegisterWallet(userID, req.AccountID, req.WalletID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"user_id":    userID,
		"account_id": req.AccountID,
		"wallet_id":  req.WalletID,
	}
	if err := h.logService.LogAction(userObjID, "RegisterWallet", "Wallet registered for trading", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "Wallet registered",
		"account_id": req.AccountID,
		"wallet_id":  req.WalletID,
	})
}

// @Summary Place a new trade
// @Description Allows an authenticated user to place a trade order on a specific account
// @Tags Trades
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trade body TradeRequest true "Trade order data"
// @Success 201 {object} map[string]interface{} "Trade placed"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Invalid account"
// @Failure 500 {object} map[string]string "Server error"
// @Router /trades [post]
func (h *TradeHandler) PlaceTrade(c *gin.Context) {
	var req TradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if req.OrderType == "MARKET" && req.EntryPrice > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "EntryPrice not allowed for MARKET orders"})
		return
	}
	if strings.Contains(req.OrderType, "LIMIT") || strings.Contains(req.OrderType, "STOP") {
		if req.EntryPrice <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "EntryPrice required for LIMIT/STOP orders"})
			return
		}
	}

	userID := c.GetString("user_id")
	trade, tradeResponse, err := h.tradeService.PlaceTrade(userID, req.AccountID, req.SymbolName, req.AccountType, req.TradeType, req.OrderType, req.Leverage, req.Volume, req.EntryPrice, req.StopLoss, req.TakeProfit, req.Expiration)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	executionType := models.ExecutionTypeUserToUser
	if req.OrderType == "MARKET" {
		executionType = models.ExecutionTypePlatform
	}

	metadata := map[string]interface{}{
		"user_id":        userID,
		"account_id":     req.AccountID,
		"trade_id":       trade.ID.Hex(),
		"symbol":         req.SymbolName,
		"trade_type":     req.TradeType,
		"order_type":     req.OrderType,
		"execution_type": executionType,
	}
	userObjID, _ := primitive.ObjectIDFromHex(userID)
	if err := h.logService.LogAction(userObjID, "PlaceTrade", "Trade order placed", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":           "Trade placed",
		"trade_id":         trade.ID.Hex(),
		"account_id":       req.AccountID,
		"trade_status":     trade.Status,
		"matched_trade_id": trade.MatchedTradeID,
		"mt5_response":     tradeResponse,
		"execution_type":   executionType,
	})
}

// @Summary Close a trade
// @Description Allows an authenticated user to close an open trade
// @Tags Trades
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Trade ID"
// @Success 200 {object} map[string]interface{} "Trade close requested"
// @Failure 400 {object} map[string]string "Invalid trade ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden (trade belongs to another user or account)"
// @Failure 404 {object} map[string]string "Trade not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /trades/{id}/close [put]
func (h *TradeHandler) CloseTrade(c *gin.Context) {
	tradeID := c.Param("id")
	userID := c.GetString("user_id")
	trade, err := h.tradeService.GetTrade(tradeID)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trade ID"})
		return
	}
	if trade == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	if trade.UserID.Hex() != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden (trade belongs to another user)"})
		return
	}

	closeResponse, err := h.tradeService.CloseTrade(tradeID, userID, trade.AccountType, trade.AccountID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	metadata := map[string]interface{}{
		"user_id":    userID,
		"account_id": trade.AccountID.Hex(),
		"trade_id":   tradeID,
	}
	if err := h.logService.LogAction(userObjID, "CloseTrade", "Trade close requested", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "Trade closed",
		"trade_id":     tradeID,
		"account_id":   trade.AccountID.Hex(),
		"mt5_response": closeResponse,
	})
}

// @Summary Stream user trades
// @Description Initiates streaming of a user's trade orders
// @Tags Trades
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Streaming started"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /trades/stream [get]
func (h *TradeHandler) StreamTrades(c *gin.Context) {
	userID := c.GetString("user_id")
	accountType := c.GetString("account_type")

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}
	if accountType != "DEMO" && accountType != "REAL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account type"})
		return
	}

	conn, err := ws.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade to WebSocket"})
		return
	}

	client := h.hub.RegisterClient(conn)

	subscriptionKey := userID + ":" + accountType
	client.Subscribe(subscriptionKey)

	metadata := map[string]interface{}{
		"user_id":      userID,
		"account_type": accountType,
	}
	if err := h.logService.LogAction(userObjID, "StreamTrades", "Trade streaming started", c.ClientIP(), metadata); err != nil {
		log.Printf("Failed to log stream action: %v", err)
	}

	if _, err := h.tradeService.StreamTrades(userID, accountType); err != nil {
		client.Conn.WriteJSON(models.ErrorResponse{Error: err.Error()})
		h.hub.UnregisterClient(client)
		return
	}

	if err := client.Conn.WriteJSON(map[string]string{
		"status":       "trade_stream_started",
		"user_id":      userID,
		"account_type": accountType,
	}); err != nil {
		h.hub.UnregisterClient(client)
		return
	}
}

// @Summary Get user trades
// @Description Retrieves a list of trades for the authenticated user
// @Tags Trades
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.TradeHistory
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /trades [get]
func (h *TradeHandler) GetUserTrades(c *gin.Context) {
	userID := c.GetString("user_id")
	trades, err := h.tradeService.GetTradesByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve trades"})
		return
	}

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	metadata := map[string]interface{}{
		"user_id": userID,
		"count":   len(trades),
	}
	if err := h.logService.LogAction(userObjID, "GetUserTrades", "Retrieved user trades", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, trades)
}

// @Summary Get trade by ID
// @Description Retrieves details of a specific trade by its ID (user or admin)
// @Tags Trades
// @Produce json
// @Security BearerAuth
// @Param id path string true "Trade ID"
// @Success 200 {object} models.TradeHistory
// @Failure 400 {object} map[string]string "Invalid trade ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden (trade belongs to another user)"
// @Failure 404 {object} map[string]string "Trade not found"
// @Router /trades/{id} [get]
func (h *TradeHandler) GetTrade(c *gin.Context) {
	tradeID := c.Param("id")
	userID := c.GetString("user_id")

	trade, err := h.tradeService.GetTrade(tradeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trade ID"})
		return
	}
	if trade == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	metadata := map[string]interface{}{
		"user_id":    userID,
		"account_id": trade.AccountID.Hex(),
		"trade_id":   tradeID,
	}
	if err := h.logService.LogAction(userObjID, "GetTrade", "Trade data retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, trade)
}

// @Summary Handle trade response from MT5
// @Description Processes trade response from MT5 EA
// @Tags Trades
// @Accept json
// @Produce json
// @Param response body interfaces.TradeResponse true "Trade response data"
// @Success 200 {object} map[string]string "Response processed"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 500 {object} map[string]string "Server error"
// @Router /trade-response [post]
func (h *TradeHandler) HandleTradeResponse(c *gin.Context) {
	var response interfaces.TradeResponse
	if err := c.ShouldBindJSON(&response); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := h.tradeService.HandleTradeResponse(response); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Response processed"})
}

// @Summary Get all trades
// @Description Retrieves a list of all trades (admin only)
// @Tags Trades
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.TradeHistory
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden (non-admin)"
// @Failure 500 {object} map[string]string "Server error"
// @Router /admin/trades [get]
func (h *TradeHandler) GetAllTrades(c *gin.Context) {
	trades, err := h.tradeService.GetAllTrades()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve trades"})
		return
	}

	userID := c.GetString("user_id")
	userObjID, _ := primitive.ObjectIDFromHex(userID)
	metadata := map[string]interface{}{
		"admin_id": userID,
		"count":    len(trades),
	}
	if err := h.logService.LogAction(userObjID, "GetAllTrades", "Retrieved all trades", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, trades)
}

// @Summary Modify a pending trade
// @Description Modify the entry price and/or volume of a pending trade
// @Tags Trades
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Trade ID"
// @Param request body ModifyTradeRequest true "Modify trade request"
// @Success 200 {object} interfaces.TradeResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 408 {object} map[string]string
// @Router /trades/{id}/modify [put]
func (h *TradeHandler) ModifyTrade(c *gin.Context) {
	tradeID := c.Param("id")
	userID := c.GetString("user_id")

	var req ModifyTradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	response, err := h.tradeService.ModifyTrade(c.Request.Context(), userID, tradeID, req.AccountType, req.AccountID, req.EntryPrice, req.Volume)
	if err != nil {
		if err.Error() == "timeout waiting for modify response" {
			c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

type WalletRequest struct {
	AccountID string `json:"account_id" binding:"required"`
	WalletID  string `json:"wallet_id" binding:"required"`
}

type ModifyTradeRequest struct {
	EntryPrice  float64 `json:"entry_price" binding:"omitempty,gt=0"`
	Volume      float64 `json:"volume" binding:"omitempty,gt=0"`
	AccountType string  `json:"account_type" binding:"required"`
	AccountID   string  `json:"account_id" binding:"required"`
}

type TradeRequest struct {
	SymbolName  string           `json:"symbol_name" binding:"required"`
	TradeType   models.TradeType `json:"trade_type" binding:"required,oneof=BUY SELL"`
	OrderType   string           `json:"order_type" binding:"required,oneof=MARKET BUY_STOP SELL_STOP BUY_LIMIT SELL_LIMIT"`
	Leverage    int              `json:"leverage" binding:"required,gt=0"`
	Volume      float64          `json:"volume" binding:"required,gt=0"`
	EntryPrice  float64          `json:"entry_price" binding:"omitempty,gt=0"`
	StopLoss    float64          `json:"stop_loss" binding:"omitempty,gte=0"`
	TakeProfit  float64          `json:"take_profit" binding:"omitempty,gte=0"`
	Expiration  *time.Time       `json:"expiration" binding:"omitempty"`
	AccountType string           `json:"account_type" binding:"required"`
	AccountID   string           `json:"account_id" binding:"required"`
}
