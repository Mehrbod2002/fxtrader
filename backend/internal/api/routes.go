package api

import (
	"os"
	"path/filepath"

	"github.com/mehrbod2002/fxtrader/internal/config"
	"github.com/mehrbod2002/fxtrader/internal/middleware"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/service"
	"github.com/mehrbod2002/fxtrader/internal/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(r *gin.Engine, cfg *config.Config, alertService service.AlertService, copyTradeService service.CopyTradeService, priceService service.PriceService, adminRepo repository.AdminRepository, userService service.UserService, symbolService service.SymbolService, logService service.LogService, ruleService service.RuleService, tradeService service.TradeService, transactionService service.TransactionService, wsHandler *ws.WebSocketHandler, baseURL string) {
	// r.SetTrustedProxies([]string{})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "User-Agent", "Referer"},
		ExposeHeaders:    []string{"Content-Length", "User-Agent", "Referer"},
		AllowCredentials: true,
	}))

	priceHandler := NewPriceHandler(priceService, logService)
	userHandler := NewUserHandler(userService, logService, cfg)
	symbolHandler := NewSymbolHandler(symbolService, logService)
	logHandler := NewLogHandler(logService)
	ruleHandler := NewRuleHandler(ruleService)
	tradeHandler := NewTradeHandler(tradeService, logService)
	transactionHandler := NewTransactionHandler(transactionService, logService)
	adminHandler := NewAdminHandler(adminRepo, cfg, userService)
	alertHandler := NewAlertHandler(alertService, logService)
	copyTradeHandler := NewCopyTradeHandler(copyTradeService, logService)

	wd, err := os.Getwd()
	if err != nil {
		return
	}

	staticPath := filepath.Join(wd, "static")
	r.Static("/static", staticPath)

	r.GET("/chart", func(c *gin.Context) {
		symbolFile := filepath.Join(staticPath, "symbol.html")
		if _, err := os.Stat(symbolFile); os.IsNotExist(err) {
			c.String(404, "symbol.html not found")
			return
		}
		c.File(symbolFile)
	})

	wdRoot := filepath.Join(wd)
	swaggerJSONPath := filepath.Join(wdRoot, "docs", "swagger.json")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/swagger.json")))
	r.GET("/docs/swagger.json", func(c *gin.Context) {
		c.File(swaggerJSONPath)
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/prices", priceHandler.HandlePrice)
		v1.POST("/users/signup", userHandler.SignupUser)
		v1.POST("/users/login", userHandler.Login)
		v1.GET("/users/:id", middleware.UserAuthMiddleware(userService), userHandler.GetUser)
		v1.GET("/symbols", symbolHandler.GetAllSymbols)
		v1.GET("/symbols/:id", symbolHandler.GetSymbol)
		v1.GET("/rules", ruleHandler.GetAllRules)
		v1.POST("/admin/login", adminHandler.AdminLogin)

		user := v1.Group("/").Use(middleware.UserAuthMiddleware(userService))
		{
			user.POST("/trades", tradeHandler.PlaceTrade)
			user.GET("/trades", tradeHandler.GetUserTrades)
			user.GET("/trades/:id", tradeHandler.GetTrade)
			user.POST("/transactions", transactionHandler.CreateTransaction)
			user.GET("/transactions", transactionHandler.GetUserTransactions)
			user.POST("/alerts", alertHandler.CreateAlert)
			user.GET("/alerts", alertHandler.GetUserAlerts)
			user.GET("/alerts/:id", alertHandler.GetAlert)
			user.POST("/copy-trades", copyTradeHandler.CreateSubscription)
			user.GET("/copy-trades", copyTradeHandler.GetUserSubscriptions)
			user.GET("/copy-trades/:id", copyTradeHandler.GetSubscription)
		}

		admin := v1.Group("/admin").Use(middleware.AdminAuthMiddleware(cfg))
		{
			admin.GET("/symbols", symbolHandler.GetAllSymbols)
			admin.POST("/symbols", symbolHandler.CreateSymbol)
			admin.PUT("/symbols/:id", symbolHandler.UpdateSymbol)
			admin.DELETE("/symbols/:id", symbolHandler.DeleteSymbol)
			admin.GET("/logs", logHandler.GetAllLogs)
			admin.GET("/logs/user/:user_id", logHandler.GetLogsByUser)
			admin.POST("/rules", ruleHandler.CreateRule)
			admin.GET("/rules", ruleHandler.GetAllRules)
			admin.GET("/rules/:id", ruleHandler.GetRule)
			admin.PUT("/rules/:id", ruleHandler.UpdateRule)
			admin.DELETE("/rules/:id", ruleHandler.DeleteRule)
			admin.GET("/users", userHandler.GetAllUsers)
			admin.GET("/trades", tradeHandler.GetAllTrades)
			admin.GET("/trades/:id", tradeHandler.GetTrade)
			admin.GET("/transactions", transactionHandler.GetAllTransactions)
			admin.GET("/transactions/id/:user_id", transactionHandler.GetTransactionByID)
			admin.GET("/transactions/user/:user_id", transactionHandler.GetTransactionsByUser)
			admin.GET("/transactions/:id", transactionHandler.GetTransactionByID)
			admin.PUT("/transactions/:id", transactionHandler.ReviewTransaction)
			admin.PUT("/users/activation", adminHandler.UpdateUserActivation)
		}
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
