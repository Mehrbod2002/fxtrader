package main

import (
	"fmt"
	"fxtrader/internal/api"
	"fxtrader/internal/config"
	"fxtrader/internal/middleware"
	"fxtrader/internal/repository"
	"fxtrader/internal/service"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	priceRepo := repository.NewPriceRepository()
	priceService := service.NewPriceService(priceRepo)

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	api.SetupRoutes(r, priceService)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
