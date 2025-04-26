package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PriceHandler struct {
	priceService service.PriceService
}

func NewPriceHandler(priceService service.PriceService) *PriceHandler {
	return &PriceHandler{priceService: priceService}
}

func (h *PriceHandler) HandlePrice(c *gin.Context) {
	var priceData models.PriceData
	if err := c.ShouldBindJSON(&priceData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := h.priceService.ProcessPrice(&priceData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process price"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Price received"})
}
