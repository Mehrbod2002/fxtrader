package api

import (
	"log"
	"net/http"

	"github.com/mehrbod2002/fxtrader/interfaces"
	"github.com/mehrbod2002/fxtrader/internal/models"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OverviewHandler struct {
	userService        service.UserService
	tradeService       interfaces.TradeService
	transactionService service.TransactionService
	symbolService      service.SymbolService
	logService         service.LogService
}

func NewOverviewHandler(
	userService service.UserService,
	tradeService interfaces.TradeService,
	transactionService service.TransactionService,
	symbolService service.SymbolService,
	logService service.LogService,
) *OverviewHandler {
	return &OverviewHandler{
		userService:        userService,
		tradeService:       tradeService,
		transactionService: transactionService,
		symbolService:      symbolService,
		logService:         logService,
	}
}

// @Summary Admin overview
// @Description Provides an overview of platform statistics
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} OverviewResponse
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden (non-admin)"
// @Failure 500 {object} map[string]string "Failed to retrieve overview data"
// @Router /admin/overview [get]
func (h *OverviewHandler) GetOverview(c *gin.Context) {
	isAdmin := c.GetBool("is_admin")
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden (non-admin)"})
		return
	}

	users, err := h.userService.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user data"})
		return
	}
	userCount := len(users)

	trades, err := h.tradeService.GetAllTrades()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve trade data"})
		return
	}

	allSymbols, err := h.symbolService.GetAllSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve symbbols"})
		return
	}

	totalTrades := len(trades)
	pendingTrades := 0
	symbolCounts := make(map[string]int)
	for _, trade := range trades {
		if trade.Status == string(models.TradeStatusPending) {
			pendingTrades++
		}
		symbolCounts[trade.Symbol]++
	}

	transactions, err := h.transactionService.GetAllTransactions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve transaction data"})
		return
	}
	totalTransactions := len(transactions)
	pendingTransactions := 0
	for _, transaction := range transactions {
		if transaction.Status == models.TransactionStatusPending {
			pendingTransactions++
		}
	}

	var topSymbols []SymbolUsage
	for symbol, count := range symbolCounts {
		topSymbols = append(topSymbols, SymbolUsage{SymbolName: symbol, TradeCount: count})
	}

	for i := 0; i < len(topSymbols)-1; i++ {
		for j := i + 1; j < len(topSymbols); j++ {
			if topSymbols[i].TradeCount < topSymbols[j].TradeCount {
				topSymbols[i], topSymbols[j] = topSymbols[j], topSymbols[i]
			}
		}
	}

	if len(topSymbols) > 5 {
		topSymbols = topSymbols[:5]
	}

	response := OverviewResponse{
		UserCount:           userCount,
		TotalTrades:         totalTrades,
		PendingTrades:       pendingTrades,
		TotalTransactions:   totalTransactions,
		PendingTransactions: pendingTransactions,
		TopSymbols:          topSymbols,
		Symbols:             len(allSymbols),
	}

	adminID := c.GetString("user_id")
	adminObjID, _ := primitive.ObjectIDFromHex(adminID)
	metadata := map[string]interface{}{
		"admin_id":             adminID,
		"user_count":           userCount,
		"total_trades":         totalTrades,
		"pending_trades":       pendingTrades,
		"total_transactions":   totalTransactions,
		"pending_transactions": pendingTransactions,
	}
	if err := h.logService.LogAction(adminObjID, "GetOverview", "Admin overview data retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, response)
}

type OverviewResponse struct {
	UserCount           int           `json:"user_count"`
	TotalTrades         int           `json:"total_trades"`
	PendingTrades       int           `json:"pending_trades"`
	TotalTransactions   int           `json:"total_transactions"`
	PendingTransactions int           `json:"pending_transactions"`
	TopSymbols          []SymbolUsage `json:"top_symbols"`
	Symbols             int           `json:"symbols"`
}

type SymbolUsage struct {
	SymbolName string `json:"symbol_name"`
	TradeCount int    `json:"trade_count"`
}
