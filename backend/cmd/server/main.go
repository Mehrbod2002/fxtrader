// cmd/server/main.go
package main

import (
	"fmt"
	"fxtrader/internal/api"
	"fxtrader/internal/config"
	"fxtrader/internal/middleware"
	"fxtrader/internal/repository"
	"fxtrader/internal/service"
	"fxtrader/internal/ws"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	api.SetupRoutes(r, priceService, wsHandler, baseURL)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on %s", addr)
	log.Printf("WebSocket endpoint available at ws://%s/ws", baseURL)
	log.Printf("Chart endpoint available at http://%s/chart?symbol=SYMBOL", baseURL)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
