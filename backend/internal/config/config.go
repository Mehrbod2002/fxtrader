package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Address    string
	Port       int
	BaseURL    string
	MongoURI   string
	AdminUser  string
	AdminPass  string
	JWTSecret  string
	MT5Host    string
	MT5Port    int
	ListenPort int
}

func Load() (*Config, error) {
	_ = godotenv.Load("../../.env")

	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "7000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, errors.New("invalid PORT value")
	}

	address := os.Getenv("ADDRESS")
	if address == "" {
		address = "0.0.0.0"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + portStr
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://admin:secret@mongodb:27017/?authSource=admin"
	}

	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}

	adminPass := os.Getenv("ADMIN_PASS")
	if adminPass == "" {
		adminPass = "admin"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default_jwt_secret"
	}

	mt5Host := os.Getenv("MT5_HOST")
	if mt5Host == "" {
		mt5Host = "127.0.0.1"
	}

	mt5PortStr := os.Getenv("MT5_PORT")
	if mt5PortStr == "" {
		mt5PortStr = "5000"
	}
	mt5Port, err := strconv.Atoi(mt5PortStr)
	if err != nil {
		return nil, errors.New("invalid MT5_PORT value")
	}

	listenPortStr := os.Getenv("LISTEN_PORT")
	if listenPortStr == "" {
		listenPortStr = "5001"
	}
	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		return nil, errors.New("invalid LISTEN_PORT value")
	}

	return &Config{
		Address:    address,
		Port:       port,
		BaseURL:    baseURL,
		MongoURI:   mongoURI,
		AdminUser:  adminUser,
		AdminPass:  adminPass,
		JWTSecret:  jwtSecret,
		MT5Host:    mt5Host,
		MT5Port:    mt5Port,
		ListenPort: listenPort,
	}, nil
}
