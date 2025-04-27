package api

import (
	"fxtrader/internal/service"
	"fxtrader/internal/ws"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, priceService service.PriceService, userService service.UserService, symbolService service.SymbolService, logService service.LogService, wsHandler *ws.WebSocketHandler, baseURL string) {
	priceHandler := NewPriceHandler(priceService, logService)
	userHandler := NewUserHandler(userService, logService)
	symbolHandler := NewSymbolHandler(symbolService, logService)
	logHandler := NewLogHandler(logService)

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
		v1.POST("/users/signup", userHandler.SignupUser)
		v1.GET("/users/:id", userHandler.GetUser)
		v1.POST("/symbols", symbolHandler.CreateSymbol)
		v1.GET("/symbols/:id", symbolHandler.GetSymbol)
		v1.GET("/symbols", symbolHandler.GetAllSymbols)
		v1.PUT("/symbols/:id", symbolHandler.UpdateSymbol)
		v1.DELETE("/symbols/:id", symbolHandler.DeleteSymbol)
		v1.GET("/logs", logHandler.GetAllLogs)
		v1.GET("/logs/user/:user_id", logHandler.GetLogsByUser)
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
