package config

import (
	"context"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func EnsureAdminUser(adminRepo repository.AdminRepository, adminUser, adminPass string) error {
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := adminRepo.GetAdminByUsername(adminUser)
	if err == nil && user != nil {
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &models.AdminAccount{
		ID:               primitive.NewObjectID(),
		Username:         adminUser,
		Password:         string(hashedPassword),
		AccountType:      "admin",
		RegistrationDate: time.Now().Format(time.RFC3339),
	}

	err = adminRepo.SaveAdmin(admin)
	if err != nil {
		return err
	}

	return nil
}
