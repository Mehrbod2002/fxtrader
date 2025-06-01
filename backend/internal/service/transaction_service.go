package service

import (
	"errors"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionService interface {
	CreateTransaction(userID string, transaction *models.Transaction) error
	GetTransactionByID(id string) (*models.Transaction, error)
	GetTransactionsByUserID(userID string) ([]*models.Transaction, error)
	GetAllTransactions() ([]*models.Transaction, error)
	ReviewTransaction(id string, status models.TransactionStatus, adminNote string) error
}

type transactionService struct {
	transactionRepo repository.TransactionRepository
	logService      LogService
}

func NewTransactionService(transactionRepo repository.TransactionRepository, logService LogService) TransactionService {
	return &transactionService{
		transactionRepo: transactionRepo,
		logService:      logService,
	}
}

func (s *transactionService) CreateTransaction(userID string, transaction *models.Transaction) error {
	if transaction.TransactionType != models.TransactionTypeDeposit && transaction.TransactionType != models.TransactionTypeWithdrawal {
		return errors.New("invalid transaction type")
	}
	if transaction.PaymentMethod != models.PaymentMethodCardToCard && transaction.PaymentMethod != models.PaymentMethodDepositReceipt {
		return errors.New("invalid payment method")
	}
	if transaction.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if transaction.PaymentMethod == models.PaymentMethodDepositReceipt && transaction.ReceiptImage == "" {
		return errors.New("receipt image required for deposit receipt method")
	}

	transaction.UserID = userID
	transaction.Status = models.TransactionStatusPending

	err := s.transactionRepo.SaveTransaction(transaction)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"transaction_id":   transaction.ID.Hex(),
		"transaction_type": transaction.TransactionType,
		"payment_method":   transaction.PaymentMethod,
		"amount":           transaction.Amount,
	}
	if err := s.logService.LogAction(primitive.ObjectID{}, "CreateTransaction", "Transaction requested", "", metadata); err != nil {
		return nil
	}

	return nil
}

func (s *transactionService) GetTransactionByID(id string) (*models.Transaction, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid transaction ID")
	}
	return s.transactionRepo.GetTransactionByID(objID)
}

func (s *transactionService) GetTransactionsByUserID(userID string) ([]*models.Transaction, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}
	return s.transactionRepo.GetTransactionsByUserID(objID)
}

func (s *transactionService) GetAllTransactions() ([]*models.Transaction, error) {
	return s.transactionRepo.GetAllTransactions()
}

func (s *transactionService) ReviewTransaction(id string, status models.TransactionStatus, adminNote string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid transaction ID")
	}

	transaction, err := s.transactionRepo.GetTransactionByID(objID)
	if err != nil {
		return err
	}
	if transaction == nil {
		return errors.New("transaction not found")
	}

	if status != models.TransactionStatusApproved && status != models.TransactionStatusRejected {
		return errors.New("invalid status; must be APPROVED or REJECTED")
	}

	if transaction.Status != models.TransactionStatusPending {
		return errors.New("transaction already reviewed")
	}

	responseTime := time.Now()
	transaction.Status = status
	transaction.ResponseTime = &responseTime
	transaction.AdminNote = adminNote

	err = s.transactionRepo.UpdateTransaction(objID, transaction)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"transaction_id": id,
		"status":         status,
		"admin_note":     adminNote,
	}
	if err := s.logService.LogAction(primitive.ObjectID{}, "ReviewTransaction", "Transaction reviewed", "", metadata); err != nil {
		return nil
	}

	return nil
}
