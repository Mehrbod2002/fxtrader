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
	"os"
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

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}

	hub := ws.NewHub()
	go hub.Run()

	wsHandler := ws.NewWebSocketHandler(hub)

	priceRepo := repository.NewPriceRepository()
	priceService := service.NewPriceService(priceRepo, hub)

	userRepo := repository.NewUserRepository(client, "fxtrader", "users")
	userService := service.NewUserService(userRepo)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	api.SetupRoutes(r, priceService, userService, wsHandler, baseURL)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on %s", addr)
	log.Printf("WebSocket endpoint available at ws://%s/ws", baseURL)
	log.Printf("Chart endpoint available at http://%s/chart?symbol=SYMBOL", baseURL)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
