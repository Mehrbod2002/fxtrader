package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port int
}

func Load() (*Config, error) {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	return &Config{
		Port: port,
	}, nil
}
