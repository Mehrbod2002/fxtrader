package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Address string
	Port    int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	address := os.Getenv("ADDRESS")
	if address == "" {
		address = "0.0.0.0"
	}

	return &Config{
		Address: address,
		Port:    port,
	}, nil
}
