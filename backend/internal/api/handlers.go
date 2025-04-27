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

// HandlePrice processes new price data
// @Summary Process new price data
// @Description Receives and processes price data for a trading symbol
// @Tags Prices
// @Accept json
// @Produce json
// @Param priceData body models.PriceData true "Price data"
// @Success 200 {object} map[string]string "Price received"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 500 {object} map[string]string "Failed to process price"
// @Router /prices [post]
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
