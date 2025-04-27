package service

import (
	"fxtrader/internal/models"
	"fxtrader/internal/repository"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService interface {
	SignupUser(user *models.UserAccount) error
	GetUser(id string) (*models.UserAccount, error)
	GetUserByTelegramID(telegramID string) (*models.UserAccount, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) SignupUser(user *models.UserAccount) error {
	user.ID = primitive.NewObjectID()
	user.RegistrationDate = time.Now().Format(time.RFC3339)
	return s.userRepo.SaveUser(user)
}

func (s *userService) GetUser(id string) (*models.UserAccount, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.userRepo.GetUserByID(objID)
}

func (s *userService) GetUserByTelegramID(telegramID string) (*models.UserAccount, error) {
	return s.userRepo.GetUserByTelegramID(telegramID)
}
