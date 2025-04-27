package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SymbolHandler struct {
	symbolService service.SymbolService
}

func NewSymbolHandler(symbolService service.SymbolService) *SymbolHandler {
	return &SymbolHandler{symbolService: symbolService}
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
	c.JSON(http.StatusOK, symbol)
}

func (h *SymbolHandler) GetAllSymbols(c *gin.Context) {
	symbols, err := h.symbolService.GetAllSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve symbols"})
		return
	}
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
	c.JSON(http.StatusOK, gin.H{"status": "Symbol updated"})
}

func (h *SymbolHandler) DeleteSymbol(c *gin.Context) {
	id := c.Param("id")
	if err := h.symbolService.DeleteSymbol(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete symbol"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "Symbol deleted"})
}
