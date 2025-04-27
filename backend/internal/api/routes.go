package api

import (
	"fxtrader/internal/config"
	"fxtrader/internal/middleware"
	"fxtrader/internal/service"
	"fxtrader/internal/ws"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title FXTrader API
// @version 1.0
// @description API for the FXTrader app
// @host localhost:8080
// @BasePath /api
// @schemes http
// @securityDefinitions.basic BasicAuth
func SetupRoutes(r *gin.Engine, cfg *config.Config, priceService service.PriceService, userService service.UserService, symbolService service.SymbolService, logService service.LogService, ruleService service.RuleService, wsHandler *ws.WebSocketHandler, baseURL string) {
	priceHandler := NewPriceHandler(priceService, logService)
	userHandler := NewUserHandler(userService, logService)
	symbolHandler := NewSymbolHandler(symbolService, logService)
	logHandler := NewLogHandler(logService)
	ruleHandler := NewRuleHandler(ruleService)

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

	wdRoot := filepath.Join(wd, "..", "..")
	swaggerJSONPath := filepath.Join(wdRoot, "docs", "swagger.json")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/swagger.json")))
	r.GET("/docs/swagger.json", func(c *gin.Context) {
		c.File(swaggerJSONPath)
	})

	v1 := r.Group("/api")
	{
		v1.POST("/prices", priceHandler.HandlePrice)
		v1.POST("/users/signup", userHandler.SignupUser)
		v1.GET("/users/:id", userHandler.GetUser)
		v1.GET("/symbols", symbolHandler.GetAllSymbols)
		v1.GET("/symbols/:id", symbolHandler.GetSymbol)
		v1.GET("/rules", ruleHandler.GetAllRules)

		admin := v1.Group("/admin").Use(middleware.AdminAuthMiddleware(cfg))
		{
			admin.POST("/symbols", symbolHandler.CreateSymbol)
			admin.PUT("/symbols/:id", symbolHandler.UpdateSymbol)
			admin.DELETE("/symbols/:id", symbolHandler.DeleteSymbol)
			admin.GET("/logs", logHandler.GetAllLogs)
			admin.GET("/logs/user/:user_id", logHandler.GetLogsByUser)
			admin.POST("/rules", ruleHandler.CreateRule)
			admin.GET("/rules/:id", ruleHandler.GetRule)
			admin.PUT("/rules/:id", ruleHandler.UpdateRule)
			admin.DELETE("/rules/:id", ruleHandler.DeleteRule)
		}
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
