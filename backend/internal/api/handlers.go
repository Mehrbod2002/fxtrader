package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PriceHandler struct {
	priceService service.PriceService
	logService   service.LogService
}

func NewPriceHandler(priceService service.PriceService, logService service.LogService) *PriceHandler {
	return &PriceHandler{priceService: priceService, logService: logService}
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

	metadata := map[string]interface{}{
		"symbol":    priceData.Symbol,
		"ask":       priceData.Ask,
		"bid":       priceData.Bid,
		"timestamp": priceData.Timestamp,
	}
	h.logService.LogAction(primitive.ObjectID{}, "ProcessPrice", "Processed new price data", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, gin.H{"status": "Price received"})
}
