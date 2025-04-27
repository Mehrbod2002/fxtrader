package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Address string
	Port    int
	BaseURL string
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

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + portStr
	}

	return &Config{
		Address: address,
		Port:    port,
		BaseURL: baseURL,
	}, nil
}
