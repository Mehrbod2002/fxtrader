package api

import (
	"fxtrader/internal/config"
	"fxtrader/internal/middleware"
	"fxtrader/internal/repository"
	"fxtrader/internal/service"
	"fxtrader/internal/ws"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(r *gin.Engine, cfg *config.Config, priceService service.PriceService, adminRepo repository.AdminRepository, userService service.UserService, symbolService service.SymbolService, logService service.LogService, ruleService service.RuleService, tradeService service.TradeService, transactionService service.TransactionService, wsHandler *ws.WebSocketHandler, baseURL string) {
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	priceHandler := NewPriceHandler(priceService, logService)
	userHandler := NewUserHandler(userService, logService, cfg)
	symbolHandler := NewSymbolHandler(symbolService, logService)
	logHandler := NewLogHandler(logService)
	ruleHandler := NewRuleHandler(ruleService)
	tradeHandler := NewTradeHandler(tradeService, logService)
	transactionHandler := NewTransactionHandler(transactionService, logService)
	adminHandler := NewAdminHandler(adminRepo, cfg)

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
		}

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
			admin.GET("/users", userHandler.GetAllUsers)
			admin.GET("/trades", tradeHandler.GetAllTrades)
			admin.GET("/trades/:id", tradeHandler.GetTrade)
			admin.GET("/transactions", transactionHandler.GetAllTransactions)
			admin.GET("/transactions/id/:user_id", transactionHandler.GetTransactionByID)
			admin.GET("/transactions/user/:user_id", transactionHandler.GetTransactionsByUser)
			admin.GET("/transactions/:id", transactionHandler.GetTransactionByID)
			admin.PUT("/transactions/:id", transactionHandler.ReviewTransaction)
		}
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
