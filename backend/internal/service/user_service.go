package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService interface {
	SignupUser(user *models.UserAccount) error
	EditUser(user *models.UserAccount) error
	GetUser(id string) (*models.UserAccount, error)
	GetUserByTelegramID(telegramID string) (*models.UserAccount, error)
	GetUsersByLeaderStatus(isLeader bool) ([]*models.UserAccount, error)
	GetAllUsers() ([]*models.UserAccount, error)
	UpdateUser(user *models.UserAccount) error
	GetUserByReferralCode(code string) (*models.UserAccount, error)
	GetUsersReferredBy(code string, page, limit int64) ([]*models.UserAccount, int64, error)
	GetAllReferrals(page, limit int64) ([]*models.UserAccount, int64, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetUserByReferralCode(code string) (*models.UserAccount, error) {
	return s.userRepo.GetUserByReferralCode(code)
}

func (s *userService) GetUsersByLeaderStatus(isLeader bool) ([]*models.UserAccount, error) {
	return s.userRepo.GetUsersByLeaderStatus(isLeader)
}

func (s *userService) UpdateUser(user *models.UserAccount) error {
	return s.userRepo.UpdateUser(user)
}

func (s *userService) SignupUser(user *models.UserAccount) error {
	if user.ID.IsZero() {
		user.ID = primitive.NewObjectID()
		user.RegistrationDate = time.Now().Format(time.RFC3339)
		user.IsActive = false
		user.ReferralCode = uuid.New().String()[:8]
		return s.userRepo.SaveUser(user)
	}
	return s.userRepo.UpdateUser(user)
}

func (s *userService) EditUser(user *models.UserAccount) error {
	return s.userRepo.EditUser(user)
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

func (s *userService) GetAllUsers() ([]*models.UserAccount, error) {
	return s.userRepo.GetAllUsers()
}

func (s *userService) GetUsersReferredBy(code string, page, limit int64) ([]*models.UserAccount, int64, error) {
	return s.userRepo.GetUsersReferredBy(code, page, limit)
}

func (s *userService) GetAllReferrals(page, limit int64) ([]*models.UserAccount, int64, error) {
	return s.userRepo.GetAllReferrals(page, limit)
}
