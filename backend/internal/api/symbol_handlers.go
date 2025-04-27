package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

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

	metadata := map[string]interface{}{
		"symbol_id":   symbol.ID.Hex(),
		"symbol_name": symbol.SymbolName,
	}
	h.logService.LogAction(primitive.ObjectID{}, "CreateSymbol", "Symbol created", c.ClientIP(), metadata)

	c.JSON(http.StatusCreated, gin.H{"status": "Symbol created", "symbol_id": symbol.ID.Hex()})
}

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

func (h *SymbolHandler) GetAllSymbols(c *gin.Context) {
	symbols, err := h.symbolService.GetAllSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve symbols"})
		return
	}

	h.logService.LogAction(primitive.ObjectID{}, "GetAllSymbols", "All symbols retrieved", c.ClientIP(), nil)

	c.JSON(http.StatusOK, symbols)
}

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
