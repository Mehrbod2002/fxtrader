package api

import (
	"fxtrader/internal/service"
	"fxtrader/internal/ws"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, priceService service.PriceService, wsHandler *ws.WebSocketHandler, baseURL string) {
	priceHandler := NewPriceHandler(priceService)

	wd, err := os.Getwd()
	if err != nil {
		return
	}

	staticPath := filepath.Join(wd, "..", "..", "static")

	r.Static("/static", staticPath)

	r.GET("/chart", func(c *gin.Context) {
		symbolFile := filepath.Join(staticPath, "symbol.html")

		if _, err := os.Stat(symbolFile); os.IsNotExist(err) {
			c.String(404, "symbol.html not found")
			return
		}

		c.File(symbolFile)
	})

	v1 := r.Group("/api")
	{
		v1.POST("/prices", priceHandler.HandlePrice)
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
