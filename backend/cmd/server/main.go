package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/api"
	"github.com/mehrbod2002/fxtrader/internal/config"
	"github.com/mehrbod2002/fxtrader/internal/middleware"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/service"
	"github.com/mehrbod2002/fxtrader/internal/tcp"
	"github.com/mehrbod2002/fxtrader/internal/ws"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	hub := ws.NewHub()
	go hub.Run()

	priceRepo := repository.NewPriceRepository()
	userRepo := repository.NewUserRepository(client, "fxtrader", "users_fxtrader")
	symbolRepo := repository.NewSymbolRepository(client, "fxtrader", "symbols_fxtrader")
	logRepo := repository.NewLogRepository(client, "fxtrader", "logs_fxtrader")
	ruleRepo := repository.NewRuleRepository(client, "fxtrader", "rules_fxtrader")
	tradeRepo := repository.NewTradeRepository(client, "fxtrader", "trades_fxtrader")
	transactionRepo := repository.NewTransactionRepository(client, "fxtrader", "transactions_fxtrader")
	adminRepo := repository.NewAdminRepository(client, "fxtrader", "admins_fxtrader")
	alertRepo := repository.NewAlertRepository(client, "fxtrader", "alerts")
	copyTradeRepo := repository.NewCopyTradeRepository(client, "fxtrader", "copy_trades")

	if err := config.EnsureAdminUser(adminRepo, cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("Failed to ensure admin user: %v", err)
	}

	wsHandler := ws.NewWebSocketHandler(hub)
	logService := service.NewLogService(logRepo)
	userService := service.NewUserService(userRepo)
	symbolService := service.NewSymbolService(symbolRepo)
	ruleService := service.NewRuleService(ruleRepo)
	transactionService := service.NewTransactionService(transactionRepo, logService)
	alertService := service.NewAlertService(alertRepo, symbolRepo, logService)
	copyTradeService := service.NewCopyTradeService(copyTradeRepo, nil, userService, logService)
	tradeService, err := service.NewTradeService(tradeRepo, symbolRepo, logService, copyTradeService)
	if err != nil {
		log.Fatalf("Failed to initialize trade service: %v", err)
	}
	priceService := service.NewPriceService(priceRepo, hub, alertService)

	copyTradeService.SetTradeService(tradeService)

	tcpServer, err := tcp.NewTCPServer(cfg.ListenPort)
	if err != nil {
		log.Fatalf("Failed to initialize TCP server: %v", err)
	}

	if err := tcpServer.Start(tradeService); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := alertService.ProcessTimeBasedAlerts(); err != nil {
				log.Printf("Error processing time-based alerts: %v", err)
			}
		}
	}()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	api.SetupRoutes(r, cfg, alertService, copyTradeService, priceService, adminRepo, userService, symbolService, logService, ruleService, tradeService, transactionService, wsHandler, cfg.BaseURL)

	addr := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	log.Printf("Starting server on http://%s", addr)
	log.Printf("WebSocket endpoint available at ws://%s/ws", cfg.BaseURL)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
