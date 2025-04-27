package api

import (
	"fxtrader/internal/service"
	"fxtrader/internal/ws"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, priceService service.PriceService, wsHandler *ws.WebSocketHandler) {
	priceHandler := NewPriceHandler(priceService)

	v1 := r.Group("/api")
	{
		v1.POST("/prices", priceHandler.HandlePrice)
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
