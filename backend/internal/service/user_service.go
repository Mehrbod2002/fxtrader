package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserService interface {
	SignupUser(user *models.User) error
	EditUser(user *models.User) error
	GetUser(id string) (*models.User, error)
	GetUserByTelegramID(telegramID string) (*models.User, error)
	GetUsersByLeaderStatus(isLeader bool) ([]*models.User, error)
	GetAllUsers() ([]*models.User, error)
	UpdateUser(user *models.User) error
	GetUserByReferralCode(code string) (*models.User, error)
	GetUsersReferredBy(code string, page, limit int64) ([]*models.User, int64, error)
	GetAllReferrals(page, limit int64) ([]*models.User, int64, error)
}

type AccountService interface {
	CreateAccount(account *models.Account) error
	GetAccount(id string) (*models.Account, error)
	GetAccountsByUserID(userID string) ([]*models.Account, error)
	DeleteAccount(accountID, userID primitive.ObjectID) error
}

type TransferService interface {
	TransferBalance(userID primitive.ObjectID, sourceID, destID string, amount float64, sourceType, destType string) error
}

type userService struct {
	userRepo repository.UserRepository
}

type accountService struct {
	accountRepo repository.AccountRepository
}

type transferService struct {
	userRepo    repository.UserRepository
	accountRepo repository.AccountRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func NewAccountService(accountRepo repository.AccountRepository) AccountService {
	return &accountService{accountRepo: accountRepo}
}

func NewTransferService(userRepo repository.UserRepository, accountRepo repository.AccountRepository) TransferService {
	return &transferService{userRepo: userRepo, accountRepo: accountRepo}
}

func (s *userService) GetUserByReferralCode(code string) (*models.User, error) {
	return s.userRepo.GetUserByReferralCode(code)
}

func (s *userService) GetUsersByLeaderStatus(isLeader bool) ([]*models.User, error) {
	return s.userRepo.GetUsersByLeaderStatus(isLeader)
}

func (s *userService) UpdateUser(user *models.User) error {
	return s.userRepo.UpdateUser(user)
}

func (s *userService) SignupUser(user *models.User) error {
	if user.ID.IsZero() {
		user.ID = primitive.NewObjectID()
		user.RegistrationDate = time.Now().Format(time.RFC3339)
		user.IsActive = false
		user.ReferralCode = uuid.New().String()[:8]
		user.Balance = 0.0
		user.Bonus = 0.0
	}
	return s.userRepo.SaveUser(user)
}

func (s *userService) EditUser(user *models.User) error {
	return s.userRepo.EditUser(user)
}

func (s *userService) GetUser(id string) (*models.User, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.userRepo.GetUserByID(objID)
}

func (s *userService) GetUserByTelegramID(telegramID string) (*models.User, error) {
	return s.userRepo.GetUserByTelegramID(telegramID)
}
func (s *userService) GetAllUsers() ([]*models.User, error) {
	return s.userRepo.GetAllUsers()
}

func (s *userService) GetUsersReferredBy(code string, page, limit int64) ([]*models.User, int64, error) {
	return s.userRepo.GetUsersReferredBy(code, page, limit)
}

func (s *userService) GetAllReferrals(page, limit int64) ([]*models.User, int64, error) {
	return s.userRepo.GetAllReferrals(page, limit)
}

func (s *accountService) CreateAccount(account *models.Account) error {
	if account.ID.IsZero() {
		account.ID = primitive.NewObjectID()
		account.RegistrationDate = time.Now().Format(time.RFC3339)
		account.IsActive = false
	}
	if account.AccountType != "demo" && account.AccountType != "real" {
		return fmt.Errorf("invalid account type: %s", account.AccountType)
	}
	return s.accountRepo.SaveAccount(account)
}

func (s *accountService) GetAccount(id string) (*models.Account, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.accountRepo.GetAccountByID(objID)
}

func (s *accountService) GetAccountsByUserID(userID string) ([]*models.Account, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	return s.accountRepo.GetAccountsByUserID(objID)
}

func (s *accountService) DeleteAccount(accountID, userID primitive.ObjectID) error {
	return s.accountRepo.DeleteAccount(accountID, userID)
}

func (s *transferService) TransferBalance(userID primitive.ObjectID, sourceID, destID string, amount float64, sourceType, destType string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := s.userRepo.Collection().Database().Client().StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {
		var sourceUser *models.User
		var sourceAccount *models.Account
		var sourceBalance *float64

		if sourceType == "main" {
			sourceUser, err = s.userRepo.GetUserByID(userID)
			if err != nil || sourceUser == nil {
				return nil, fmt.Errorf("source user not found")
			}
			sourceBalance = &sourceUser.Balance
		} else {
			sourceAccount, err = s.accountRepo.GetAccountByName(sourceID)
			if err != nil || sourceAccount == nil {
				return nil, fmt.Errorf("source account not found")
			}
			if sourceAccount.AccountType != sourceType {
				return nil, fmt.Errorf("source account type mismatch: expected %s, got %s", sourceType, sourceAccount.AccountType)
			}
			sourceBalance = &sourceAccount.Balance
			sourceUser, err = s.userRepo.GetUserByID(sourceAccount.UserID)
			if err != nil || sourceUser == nil {
				return nil, fmt.Errorf("source user not found")
			}
		}

		var destUser *models.User
		var destAccount *models.Account
		var destBalance *float64

		if destType == "main" {
			destUser, err = s.userRepo.GetUserByID(userID)
			if err != nil || destUser == nil {
				return nil, fmt.Errorf("destination user not found")
			}
			destBalance = &destUser.Balance
		} else {
			destAccount, err = s.accountRepo.GetAccountByName(destID)
			if err != nil || destAccount == nil {
				return nil, fmt.Errorf("destination account not found")
			}
			if destAccount.AccountType != destType {
				return nil, fmt.Errorf("destination account type mismatch: expected %s, got %s", destType, destAccount.AccountType)
			}
			destBalance = &destAccount.Balance
			destUser, err = s.userRepo.GetUserByID(destAccount.UserID)
			if err != nil || destUser == nil {
				return nil, fmt.Errorf("destination user not found")
			}
		}

		if sourceUser.ID != destUser.ID {
			return nil, fmt.Errorf("transfers must be within the same user")
		}

		if (sourceType == "demo" && destType == "real") || (sourceType == "real" && destType == "demo") {
			return nil, fmt.Errorf("cannot transfer between demo and real balances")
		}

		if *sourceBalance < amount {
			return nil, fmt.Errorf("insufficient balance in source account")
		}

		*sourceBalance -= amount
		*destBalance += amount

		if sourceType == "main" {
			if err := s.userRepo.UpdateUser(sourceUser); err != nil {
				return nil, fmt.Errorf("failed to update source user: %w", err)
			}
		} else {
			if err := s.accountRepo.UpdateAccount(sourceAccount); err != nil {
				return nil, fmt.Errorf("failed to update source account: %w", err)
			}
		}

		if destType == "main" {
			if err := s.userRepo.UpdateUser(destUser); err != nil {
				return nil, fmt.Errorf("failed to update destination user: %w", err)
			}
		} else {
			if err := s.accountRepo.UpdateAccount(destAccount); err != nil {
				return nil, fmt.Errorf("failed to update destination account: %w", err)
			}
		}

		return nil, nil
	}

	_, err = session.WithTransaction(ctx, callback)
	return err
}
