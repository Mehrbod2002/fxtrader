package api

import (
	"log"
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionHandler struct {
	accountRepo        repository.UserRepository
	transactionService service.TransactionService
	logService         service.LogService
}

func NewTransactionHandler(transactionService service.TransactionService, logService service.LogService, accountRepo repository.UserRepository) *TransactionHandler {
	return &TransactionHandler{transactionService: transactionService, logService: logService, accountRepo: accountRepo}
}

// @Summary Request a new transaction
// @Description Allows a user to request a deposit or withdrawal
// @Tags Transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param transaction body TransactionRequest true "Transaction data"
// @Success 201 {object} map[string]string "Transaction requested"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to create transaction"
// @Router /transactions [post]
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	var req TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID := c.GetString("user_id")
	userObjID, _ := primitive.ObjectIDFromHex(userID)
	user, _ := h.accountRepo.GetUserByID(userObjID)
	transaction := &models.Transaction{
		TransactionType: req.TransactionType,
		PaymentMethod:   req.PaymentMethod,
		Amount:          req.Amount,
		TelegramID:      user.TelegramID,
		ReceiptImage:    req.ReceiptImage,
	}

	if err := h.transactionService.CreateTransaction(userID, transaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Transaction requested", "transaction_id": transaction.ID.Hex()})
}

// @Summary Get user transactions
// @Description Retrieves all transactions for the authenticated user
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Transaction
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to retrieve transactions"
// @Router /transactions [get]
func (h *TransactionHandler) GetUserTransactions(c *gin.Context) {
	userID := c.GetString("user_id")
	transactions, err := h.transactionService.GetTransactionsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// @Summary Get all transactions
// @Description Retrieves a list of all transactions (admin only)
// @Tags Transactions
// @Produce json
// @Security BasicAuth
// @Success 200 {array} models.Transaction
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to retrieve transactions"
// @Router /admin/transactions [get]
func (h *TransactionHandler) GetAllTransactions(c *gin.Context) {
	transactions, err := h.transactionService.GetAllTransactions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve transactions"})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// @Summary Get transactions by user ID
// @Description Retrieves transactions for a specific user (admin only)
// @Tags Transactions
// @Produce json
// @Security BasicAuth
// @Param user_id path string true "User ID"
// @Success 200 {array} models.Transaction
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to retrieve transactions"
// @Router /admin/transactions/user/{user_id} [get]
func (h *TransactionHandler) GetTransactionsByUser(c *gin.Context) {
	userID := c.Param("user_id")
	transactions, err := h.transactionService.GetTransactionsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// @Summary Get transaction by ID
// @Description Retrieves details of a specific transaction by its ID (admin only)
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Success 200 {object} models.Transaction
// @Failure 400 {object} map[string]string "Invalid transaction ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden (non-admin)"
// @Failure 404 {object} map[string]string "Transaction not found"
// @Router /api/v1/transactions/{id} [get]
func (h *TransactionHandler) GetTransactionByID(c *gin.Context) {
	transactionID := c.Param("id")
	transaction, err := h.transactionService.GetTransactionByID(transactionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}
	if transaction == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	adminID := c.GetString("user_id")
	adminObjID, _ := primitive.ObjectIDFromHex(adminID)
	metadata := map[string]interface{}{
		"admin_id":       adminID,
		"transaction_id": transactionID,
	}
	if err := h.logService.LogAction(adminObjID, "GetTransactionByID", "Transaction data retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, transaction)
}

// @Summary Approve a transaction
// @Description Approves a transaction with a reason and admin comment, updating user balance (admin only)
// @Tags Transactions
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param id path string true "Transaction ID"
// @Param review body TransactionReviewRequest true "Approval data"
// @Success 200 {object} map[string]string "Transaction approved"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to approve transaction"
// @Router /admin/transactions/{id}/approve [put]
func (h *TransactionHandler) ApproveTransaction(c *gin.Context) {
	id := c.Param("id")
	var req TransactionReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := h.transactionService.ApproveTransaction(id, req.Reason, req.AdminComment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transaction, _ := h.transactionService.GetTransactionByID(id)
	status := "Transaction approved"
	if transaction != nil {
		switch transaction.TransactionType {
		case models.TransactionTypeDeposit:
			status = "Deposit approved"
		case models.TransactionTypeWithdrawal:
			status = "Withdrawal approved"
		}
	}

	metadata := map[string]interface{}{
		"transaction_id": id,
		"reason":         req.Reason,
		"admin_comment":  req.AdminComment,
		"status":         status,
		"amount":         transaction.Amount,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "ApproveTransaction", status, c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

// @Summary Deny a transaction
// @Description Rejects a transaction with a reason and admin comment (admin only)
// @Tags Transactions
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param id path string true "Transaction ID"
// @Param review body TransactionReviewRequest true "Denial data"
// @Success 200 {object} map[string]string "Transaction denied"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to deny transaction"
// @Router /admin/transactions/{id}/deny [put]
func (h *TransactionHandler) DenyTransaction(c *gin.Context) {
	id := c.Param("id")
	var req TransactionReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := h.transactionService.DenyTransaction(id, req.Reason, req.AdminComment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transaction, _ := h.transactionService.GetTransactionByID(id)
	status := "Transaction denied"
	if transaction != nil {
		switch transaction.TransactionType {
		case models.TransactionTypeDeposit:
			status = "Deposit denied"
		case models.TransactionTypeWithdrawal:
			status = "Withdrawal denied"
		}
	}

	metadata := map[string]interface{}{
		"transaction_id": id,
		"reason":         req.Reason,
		"admin_comment":  req.AdminComment,
		"status":         status,
		"amount":         transaction.Amount,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "DenyTransaction", status, c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

type TransactionRequest struct {
	TransactionType models.TransactionType `json:"transaction_type" binding:"required,oneof=DEPOSIT WITHDRAWAL"`
	PaymentMethod   models.PaymentMethod   `json:"payment_method" binding:"required,oneof=CARD_TO_CARD DEPOSIT_RECEIPT"`
	Amount          float64                `json:"amount" binding:"required,gt=0"`
	ReceiptImage    string                 `json:"receipt_image,omitempty"`
}

type TransactionReviewRequest struct {
	Reason       string `json:"reason" binding:"required"`
	AdminComment string `json:"admin_comment" binding:"required"`
}
