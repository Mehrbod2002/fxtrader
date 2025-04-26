package api

import (
	"fxtrader/internal/service"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, priceService service.PriceService) {
	priceHandler := NewPriceHandler(priceService)

	v1 := r.Group("/api")
	{
		v1.POST("/prices", priceHandler.HandlePrice)
	}
}
