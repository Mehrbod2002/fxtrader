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
	defer client.Disconnect(context.Background())

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

	wsHandler := ws.NewWebSocketHandler(hub)
	priceService := service.NewPriceService(priceRepo, hub)
	userService := service.NewUserService(userRepo)
	symbolService := service.NewSymbolService(symbolRepo)
	logService := service.NewLogService(logRepo)
	ruleService := service.NewRuleService(ruleRepo)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	api.SetupRoutes(r, cfg, priceService, userService, symbolService, logService, ruleService, wsHandler, cfg.BaseURL)

	addr := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	log.Printf("Starting server on http://%s", addr)
	log.Printf("WebSocket endpoint available at ws://%s/ws", cfg.BaseURL)
	log.Printf("Chart endpoint available at http://%s/chart?symbol=SYMBOL", cfg.BaseURL)
	log.Printf("Swagger UI available at http://%s/swagger/index.html", cfg.BaseURL)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
