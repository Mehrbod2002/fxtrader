package api

import (
	"log"
	"os"
	"path/filepath"

	"github.com/mehrbod2002/fxtrader/interfaces"
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

func SetupRoutes(
	r *gin.Engine,
	cfg *config.Config,
	alertService service.AlertService,
	copyTradeService service.CopyTradeService,
	priceService service.PriceService,
	adminRepo repository.AdminRepository,
	userService service.UserService,
	symbolService service.SymbolService,
	logService service.LogService,
	ruleService service.RuleService,
	tradeService interfaces.TradeService,
	transactionService service.TransactionService,
	wsHandler *ws.WebSocketHandler,
	hub *ws.Hub,
	leaderRequestService service.LeaderRequestService,
) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "User-Agent", "Referer", "X-Telegram-ID"},
		ExposeHeaders:    []string{"Content-Length", "User-Agent", "Referer"},
		AllowCredentials: true,
	}))

	priceHandler := NewPriceHandler(priceService, logService)
	userHandler := NewUserHandler(userService, logService, cfg)
	symbolHandler := NewSymbolHandler(symbolService, logService)
	logHandler := NewLogHandler(logService)
	overviewHandler := NewOverviewHandler(userService, tradeService, transactionService, symbolService, logService)
	ruleHandler := NewRuleHandler(ruleService)
	tradeHandler := NewTradeHandler(tradeService, logService, hub)
	transactionHandler := NewTransactionHandler(transactionService, logService)
	adminHandler := NewAdminHandler(adminRepo, cfg, userService)
	alertHandler := NewAlertHandler(alertService, logService)
	copyTradeHandler := NewCopyTradeHandler(copyTradeService, logService)
	leaderRequestHandler := NewLeaderRequestHandler(leaderRequestService, logService)

	wd, err := os.Getwd()
	if err != nil {
		return
	}

	staticPath := filepath.Join(wd, "static")
	r.Static("/static", staticPath)

	r.GET("/chart", func(c *gin.Context) {
		symbolFile := filepath.Join(staticPath, "symbol.html")
		log.Printf("Attempting to access file: %s", symbolFile)
		if _, err := os.Stat(symbolFile); os.IsNotExist(err) {
			log.Printf("File not found: %s", symbolFile)
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
		v1.GET("/users/me/:id", userHandler.GetMe)
		v1.POST("/users/login", userHandler.Login)
		v1.GET("/users/:id", middleware.UserAuthMiddleware(userService), userHandler.GetUser)
		v1.GET("/symbols", symbolHandler.GetAllSymbols)
		v1.GET("/symbols/:id", symbolHandler.GetSymbol)
		v1.GET("/rules", ruleHandler.GetAllRules)
		v1.POST("/admin/login", adminHandler.AdminLogin)
		v1.POST("/leader-requests", middleware.UserAuthMiddleware(userService), leaderRequestHandler.CreateLeaderRequest)
		v1.GET("/copy-trade-leaders", middleware.UserAuthMiddleware(userService), leaderRequestHandler.GetApprovedLeaders)
		v1.GET("/referrals", middleware.UserAuthMiddleware(userService), adminHandler.GetUserReferrals)

		user := v1.Group("/").Use(middleware.UserAuthMiddleware(userService))
		{
			user.POST("/trades", tradeHandler.PlaceTrade)
			user.GET("/trades", tradeHandler.GetUserTrades)
			user.GET("/trades/:id", tradeHandler.GetTrade)
			user.PUT("/trades/:id/close", tradeHandler.CloseTrade)
			user.GET("/trades/stream", tradeHandler.StreamTrades)
			user.PUT("/trades/:id/modify", tradeHandler.ModifyTrade)
			user.POST("/transactions", transactionHandler.CreateTransaction)
			user.GET("/transactions", transactionHandler.GetUserTransactions)
			user.POST("/alerts", alertHandler.CreateAlert)
			user.GET("/alerts", alertHandler.GetUserAlerts)
			user.GET("/alerts/:id", alertHandler.GetAlert)
			user.POST("/copy-trades", copyTradeHandler.CreateSubscription)
			user.GET("/copy-trades", copyTradeHandler.GetUserSubscriptions)
			user.GET("/copy-trades/:id", copyTradeHandler.GetSubscription)
			user.POST("/accounts", userHandler.CreateAccount)
			user.GET("/accounts", userHandler.GetUserAccounts)
			user.DELETE("/accounts/:id", userHandler.DeleteAccount)
		}

		admin := v1.Group("/admin").Use(middleware.AdminAuthMiddleware(cfg))
		{
			admin.GET("/symbols", symbolHandler.GetAllSymbols)
			admin.POST("/symbols", symbolHandler.CreateSymbol)
			admin.PUT("/symbols/:id", symbolHandler.UpdateSymbol)
			admin.DELETE("/symbols/:id", symbolHandler.DeleteSymbol)
			admin.GET("/logs", logHandler.GetAllLogs)
			admin.GET("/overview", overviewHandler.GetOverview)
			admin.GET("/logs/user/:user_id", logHandler.GetLogsByUser)
			admin.POST("/rules", ruleHandler.CreateRule)
			admin.GET("/rules", ruleHandler.GetAllRules)
			admin.GET("/rules/:id", ruleHandler.GetRule)
			admin.PUT("/rules/:id", ruleHandler.UpdateRule)
			admin.DELETE("/rules/:id", ruleHandler.DeleteRule)
			admin.GET("/users", userHandler.GetAllUsers)
			admin.GET("/users/:id", userHandler.GetMe)
			admin.PUT("/users/edit", userHandler.EditUser)
			admin.PUT("/users/activation", adminHandler.UpdateUserActivation)
			admin.GET("/trades", tradeHandler.GetAllTrades)
			admin.GET("/trades/:id", tradeHandler.GetTrade)
			admin.GET("/transactions", transactionHandler.GetAllTransactions)
			admin.GET("/transactions/id/:user_id", transactionHandler.GetTransactionByID)
			admin.GET("/transactions/user/:user_id", transactionHandler.GetTransactionsByUser)
			admin.GET("/transactions/:id", transactionHandler.GetTransactionByID)
			admin.PUT("/transactions/:id", transactionHandler.ReviewTransaction)
			admin.POST("/leader-requests/:id/approve", leaderRequestHandler.ApproveLeaderRequest)
			admin.POST("/leader-requests/:id/deny", leaderRequestHandler.DenyLeaderRequest)
			admin.GET("/leader-requests", leaderRequestHandler.GetPendingLeaderRequests)
			admin.GET("/copy-trade-leaders", leaderRequestHandler.GetApprovedLeaders)
			admin.GET("/copy-trades-all", copyTradeHandler.GetAllUserSubscriptions)
			admin.GET("/referrals", adminHandler.GetAllReferrals)
		}
	}

	r.GET("/ws", wsHandler.HandleConnection)
}
