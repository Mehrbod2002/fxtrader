package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeHandler struct {
	tradeService service.TradeService
	logService   service.LogService
}

func NewTradeHandler(tradeService service.TradeService, logService service.LogService) *TradeHandler {
	return &TradeHandler{tradeService: tradeService, logService: logService}
}

// @Summary Place a new trade
// @Description Allows an authenticated user to place a trade order
// @Tags Trades
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trade body TradeRequest true "Trade order data"
// @Success 201 {object} map[string]string "Trade placed"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/trades [post]
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
	trade, err := h.tradeService.PlaceTrade(userID, req.SymbolName, req.TradeType, req.OrderType, req.Leverage, req.Volume, req.EntryPrice, req.StopLoss, req.TakeProfit, req.Expiration)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"user_id":    userID,
		"trade_id":   trade.ID.Hex(),
		"symbol":     req.SymbolName,
		"trade_type": req.TradeType,
		"order_type": req.OrderType,
	}
	h.logService.LogAction(trade.UserID, "PlaceTrade", "Trade order placed", c.ClientIP(), metadata)

	c.JSON(http.StatusCreated, gin.H{"status": "Trade placed", "trade_id": trade.ID.Hex()})
}

// @Summary Get user trades
// @Description Retrieves a list of trades for the authenticated user
// @Tags Trades
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.TradeHistory
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/trades [get]
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
	h.logService.LogAction(userObjID, "GetUserTrades", "Retrieved user trades", c.ClientIP(), metadata)

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
// @Router /api/trades/{id} [get]
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

	// Admin
	// if trade.UserID.Hex() != userID {
	// 	c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden (trade belongs to another user)"})
	// 	return
	// }

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	metadata := map[string]interface{}{
		"user_id":  userID,
		"trade_id": tradeID,
	}
	h.logService.LogAction(userObjID, "GetTrade", "Trade data retrieved", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, trade)
}

// @Summary Handle trade response from MT5
// @Description Processes trade response from MT5 EA
// @Tags Trades
// @Accept json
// @Produce json
// @Param response body service.TradeResponse true "Trade response data"
// @Success 200 {object} map[string]string "Response processed"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/trade-response [post]
func (h *TradeHandler) HandleTradeResponse(c *gin.Context) {
	var response service.TradeResponse
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
// @Router /api/v1/admin/trades [get]
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
	h.logService.LogAction(userObjID, "GetAllTrades", "Retrieved all trades", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, trades)
}

type TradeRequest struct {
	SymbolName string           `json:"symbol_name" binding:"required"`
	TradeType  models.TradeType `json:"trade_type" binding:"required,oneof=BUY SELL"`
	OrderType  string           `json:"order_type" binding:"required,oneof=MARKET LIMIT BUY_STOP SELL_STOP BUY_LIMIT SELL_LIMIT"`
	Leverage   int              `json:"leverage" binding:"required,gt=0"`
	Volume     float64          `json:"volume" binding:"required,gt=0"`
	EntryPrice float64          `json:"entry_price" binding:"omitempty,gt=0"`
	StopLoss   float64          `json:"stop_loss" binding:"omitempty,gte=0"`
	TakeProfit float64          `json:"take_profit" binding:"omitempty,gte=0"`
	Expiration *time.Time       `json:"expiration" binding:"omitempty"`
}
