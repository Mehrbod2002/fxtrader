package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TradeHandler struct {
	tradeService service.TradeService
	logService   service.LogService
}

func NewTradeHandler(tradeService service.TradeService, logService service.LogService) *TradeHandler {
	return &TradeHandler{tradeService: tradeService, logService: logService}
}

// @Summary Place a new trade
// @Description Allows a user to place a trade order
// @Tags Trades
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param trade body TradeRequest true "Trade order data"
// @Success 201 {object} map[string]string "Trade placed"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to place trade"
// @Router /trades [post]
func (h *TradeHandler) PlaceTrade(c *gin.Context) {
	var req TradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID := c.GetString("user_id") // Set by auth middleware
	trade, err := h.tradeService.PlaceTrade(userID, req.SymbolName, req.TradeType, req.Leverage, req.Volume, req.EntryPrice)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Trade placed", "trade_id": trade.ID.Hex()})
}

// @Summary Get trade by ID
// @Description Retrieves details of a trade by ID
// @Tags Trades
// @Produce json
// @Security BasicAuth
// @Param id path string true "Trade ID"
// @Success 200 {object} models.TradeHistory
// @Failure 400 {object} map[string]string "Invalid trade ID"
// @Failure 404 {object} map[string]string "Trade not found"
// @Router /trades/{id} [get]
func (h *TradeHandler) GetTrade(c *gin.Context) {
	id := c.Param("id")
	trade, err := h.tradeService.GetTrade(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trade ID"})
		return
	}
	if trade == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	c.JSON(http.StatusOK, trade)
}

// @Summary Get user trades
// @Description Retrieves all trades for the authenticated user
// @Tags Trades
// @Produce json
// @Security BasicAuth
// @Success 200 {array} models.TradeHistory
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to retrieve trades"
// @Router /trades [get]
func (h *TradeHandler) GetUserTrades(c *gin.Context) {
	userID := c.GetString("user_id")
	trades, err := h.tradeService.GetTradesByUserID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	c.JSON(http.StatusOK, trades)
}

type TradeRequest struct {
	SymbolName string           `json:"symbol_name" binding:"required"`
	TradeType  models.TradeType `json:"trade_type" binding:"required,oneof=BUY SELL"`
	Leverage   int              `json:"leverage" binding:"required,gt=0"`
	Volume     float64          `json:"volume" binding:"required,gt=0"`
	EntryPrice float64          `json:"entry_price" binding:"required,gt=0"`
}
