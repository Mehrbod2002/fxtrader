package config

import (
	"context"
	"fxtrader/internal/models"
	"fxtrader/internal/repository"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func EnsureAdminUser(userRepo repository.UserRepository, adminUser, adminPass string) error {
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := userRepo.GetUserByTelegramID("admin_" + adminUser)
	if err == nil && user != nil {
		log.Println("Admin user already exists")
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &models.UserAccount{
		ID:               primitive.NewObjectID(),
		Username:         adminUser,
		FullName:         "Admin User",
		AccountType:      "admin",
		RegistrationDate: time.Now().Format(time.RFC3339),
		TelegramID:       "admin_" + adminUser,
		Password:         string(hashedPassword),
	}

	err = userRepo.SaveUser(admin)
	if err != nil {
		return err
	}

	log.Println("Default admin user created")
	return nil
}
