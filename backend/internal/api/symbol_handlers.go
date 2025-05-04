package api

import (
	"net/http"
	"regexp"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SymbolHandler struct {
	symbolService service.SymbolService
	logService    service.LogService
}

func NewSymbolHandler(symbolService service.SymbolService, logService service.LogService) *SymbolHandler {
	return &SymbolHandler{symbolService: symbolService, logService: logService}
}

// @Summary Create a new symbol
// @Description Adds a new trading symbol to the system (admin only)
// @Tags Symbols
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param symbol body models.Symbol true "Symbol data"
// @Success 201 {object} map[string]string "Symbol created"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to create symbol"
// @Router /admin/symbols [post]
func (h *SymbolHandler) CreateSymbol(c *gin.Context) {
	var symbol models.Symbol
	if err := c.ShouldBindJSON(&symbol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := h.symbolService.CreateSymbol(&symbol); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create symbol"})
		return
	}

	pattern := `^\d{2}:\d{2}$`
	v, _ := regexp.MatchString(pattern, symbol.TradingHours.CloseTime)
	m, _ := regexp.MatchString(pattern, symbol.TradingHours.OpenTime)
	if !v || !m {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "symbol trading hours are not valid"})
		return
	}

	metadata := map[string]interface{}{
		"symbol_id":   symbol.ID.Hex(),
		"symbol_name": symbol.SymbolName,
	}
	h.logService.LogAction(primitive.ObjectID{}, "CreateSymbol", "Symbol created", c.ClientIP(), metadata)

	c.JSON(http.StatusCreated, gin.H{"status": "Symbol created", "symbol_id": symbol.ID.Hex()})
}

// @Summary Get symbol by ID
// @Description Retrieves details of a trading symbol by ID
// @Tags Symbols
// @Produce json
// @Param id path string true "Symbol ID"
// @Success 200 {object} models.Symbol
// @Failure 400 {object} map[string]string "Invalid symbol ID"
// @Failure 404 {object} map[string]string "Symbol not found"
// @Router /symbols/{id} [get]
func (h *SymbolHandler) GetSymbol(c *gin.Context) {
	id := c.Param("id")
	symbol, err := h.symbolService.GetSymbol(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid symbol ID"})
		return
	}
	if symbol == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Symbol not found"})
		return
	}

	metadata := map[string]interface{}{
		"symbol_id": id,
	}
	h.logService.LogAction(primitive.ObjectID{}, "GetSymbol", "Symbol data retrieved", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, symbol)
}

// GetAllSymbols retrieves all symbols
// @Summary Get all symbols
// @Description Retrieves a list of all trading symbols
// @Tags Symbols
// @Produce json
// @Success 200 {array} models.Symbol
// @Failure 500 {object} map[string]string "Failed to retrieve symbols"
// @Router /symbols [get]
func (h *SymbolHandler) GetAllSymbols(c *gin.Context) {
	symbols, err := h.symbolService.GetAllSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve symbols"})
		return
	}

	h.logService.LogAction(primitive.ObjectID{}, "GetAllSymbols", "All symbols retrieved", c.ClientIP(), nil)

	c.JSON(http.StatusOK, symbols)
}

// @Summary Update a symbol
// @Description Updates the details of an existing trading symbol (admin only)
// @Tags Symbols
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param id path string true "Symbol ID"
// @Param symbol body models.Symbol true "Updated symbol data"
// @Success 200 {object} map[string]string "Symbol updated"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to update symbol"
// @Router /admin/symbols/{id} [put]
func (h *SymbolHandler) UpdateSymbol(c *gin.Context) {
	id := c.Param("id")
	var symbol models.Symbol
	if err := c.ShouldBindJSON(&symbol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := h.symbolService.UpdateSymbol(id, &symbol); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update symbol"})
		return
	}

	metadata := map[string]interface{}{
		"symbol_id": id,
	}
	h.logService.LogAction(primitive.ObjectID{}, "UpdateSymbol", "Symbol updated", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, gin.H{"status": "Symbol updated"})
}

// @Summary Delete a symbol
// @Description Removes a trading symbol from the system (admin only)
// @Tags Symbols
// @Produce json
// @Security BasicAuth
// @Param id path string true "Symbol ID"
// @Success 200 {object} map[string]string "Symbol deleted"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to delete symbol"
// @Router /admin/symbols/{id} [delete]
func (h *SymbolHandler) DeleteSymbol(c *gin.Context) {
	id := c.Param("id")
	if err := h.symbolService.DeleteSymbol(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete symbol"})
		return
	}

	metadata := map[string]interface{}{
		"symbol_id": id,
	}
	h.logService.LogAction(primitive.ObjectID{}, "DeleteSymbol", "Symbol deleted", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, gin.H{"status": "Symbol deleted"})
}
