package main

import (
	"context"
	"fmt"
	"fxtrader/internal/api"
	"fxtrader/internal/config"
	"fxtrader/internal/middleware"
	"fxtrader/internal/repository"
	"fxtrader/internal/service"
	"fxtrader/internal/ws"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
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
	userRepo := repository.NewUserRepository(client, "fxtrader", "users")
	symbolRepo := repository.NewSymbolRepository(client, "fxtrader", "symbols")
	logRepo := repository.NewLogRepository(client, "fxtrader", "logs")
	ruleRepo := repository.NewRuleRepository(client, "fxtrader", "rules")
	tradeRepo := repository.NewTradeRepository(client, "fxtrader", "trades")
	transactionRepo := repository.NewTransactionRepository(client, "fxtrader", "transactions")
	adminRepo := repository.NewAdminRepository(client, "fxtrader", "admins")

	if err := config.EnsureAdminUser(adminRepo, cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("Failed to ensure admin user: %v", err)
	}

	wsHandler := ws.NewWebSocketHandler(hub)
	priceService := service.NewPriceService(priceRepo, hub)
	userService := service.NewUserService(userRepo)
	symbolService := service.NewSymbolService(symbolRepo)
	logService := service.NewLogService(logRepo)
	ruleService := service.NewRuleService(ruleRepo)
	tradeService, err := service.NewTradeService(tradeRepo, symbolRepo, logService, cfg.MT5Host, cfg.MT5Port, cfg.ListenPort)
	if err != nil {
		log.Fatalf("Failed to initialize trade service: %v", err)
	}
	transactionService := service.NewTransactionService(transactionRepo, logService)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	api.SetupRoutes(r, cfg, priceService, adminRepo, userService, symbolService, logService, ruleService, tradeService, transactionService, wsHandler, cfg.BaseURL)

	addr := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	log.Printf("Starting server on http://%s", addr)
	log.Printf("WebSocket endpoint available at ws://%s/ws", cfg.BaseURL)
	log.Printf("Chart endpoint available at http://%s/chart?symbol=SYMBOL", cfg.BaseURL)
	log.Printf("Swagger UI available at http://%s/swagger/index.html", cfg.BaseURL)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
