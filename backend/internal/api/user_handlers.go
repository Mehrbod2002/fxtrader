package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/config"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LoginRequest struct {
	TelegramID string `json:"telegram_id" binding:"required"`
}

type CreateAccountRequest struct {
	AccountName string `json:"account_name" binding:"required"`
	AccountType string `json:"account_type" binding:"required"`
}

type TransferRequest struct {
	SourceID string  `json:"source_id" binding:"required"`
	DestID   string  `json:"dest_id" binding:"required"`
	Amount   float64 `json:"amount" binding:"required,gt=0"`
}

type UserHandler struct {
	userService service.UserService
	logService  service.LogService
	cfg         *config.Config
}

func NewUserHandler(userService service.UserService, logService service.LogService, cfg *config.Config) *UserHandler {
	return &UserHandler{userService: userService, logService: logService, cfg: cfg}
}

// @Summary Sign up a new user
// @Description Creates a new user account via Telegram
// @Tags Users
// @Accept json
// @Produce json
// @Param user body models.UserAccount true "User account details"
// @Success 201 {object} map[string]interface{} "User created"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 409 {object} map[string]string "User already exists"
// @Failure 500 {object} map[string]string "Server error"
// @Router /users/signup [post]
func (h *UserHandler) SignupUser(c *gin.Context) {
	var req models.UserAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	existingUser, err := h.userService.GetUserByTelegramID(req.TelegramID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing user"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this Telegram ID already exists"})
		return
	}

	user := &models.UserAccount{
		FullName:         req.FullName,
		PhoneNumber:      req.PhoneNumber,
		TelegramID:       req.TelegramID,
		Username:         req.Username,
		CardNumber:       req.CardNumber,
		Citizenship:      req.Citizenship,
		NationalID:       req.NationalID,
		AccountType:      "main",
		RegistrationDate: time.Now().Format(time.RFC3339),
	}

	if user.Username == "" {
		user.Username = "user_" + user.TelegramID
	}

	var referredBy primitive.ObjectID
	if req.ReferralCode != "" {
		referrer, err := h.userService.GetUserByReferralCode(req.ReferralCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate referral code"})
			return
		}
		if referrer == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid referral code"})
			return
		}
		referredBy = referrer.ID
	}

	timestamp := time.Now().UnixNano()
	user.ReferralCode = fmt.Sprintf("%s-%x", user.Username, timestamp)[0:12]
	user.ReferredBy = referredBy

	if err := h.userService.SignupUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	metadata := map[string]interface{}{
		"username": user.Username,
		"user_id":  user.ID.Hex(),
	}
	if err := h.logService.LogAction(user.ID, "UserSignup", "User signed up via Telegram", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "User created",
		"user_id": user.ID.Hex(),
		"user":    user,
	})
}

// @Summary Create a new user account
// @Description Creates a new user account with only an account name
// @Tags Users
// @Accept json
// @Produce json
// @Param account body CreateAccountRequest true "Account name"
// @Success 201 {object} map[string]interface{} "Account created"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /accounts [post]
func (h *UserHandler) CreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	account := &models.UserAccount{
		AccountName: req.AccountName,
		AccountType: req.AccountType,
		TelegramID:  userID.(string),
	}

	if err := h.userService.CreateAccount(account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	metadata := map[string]interface{}{
		"account_name": req.AccountName,
		"user_id":      userID,
	}
	if err := h.logService.LogAction(userObjID, "CreateAccount", "User account created", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":       "Account created",
		"account_id":   account.ID.Hex(),
		"account_name": account.AccountName,
	})
}

// @Summary Get user accounts
// @Description Retrieves a list of accounts for the authenticated user
// @Tags Users
// @Produce json
// @Success 200 {array} models.UserAccount
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /accounts [get]
func (h *UserHandler) GetUserAccounts(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	accounts, err := h.userService.GetUserAccounts(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve accounts"})
		return
	}

	metadata := map[string]interface{}{
		"user_id": userID,
		"count":   len(accounts),
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetUserAccounts", "Retrieved user accounts", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, accounts)
}

// @Summary Delete user account
// @Description Deletes a user account by its ID
// @Tags Users
// @Produce json
// @Param id path string true "Account ID"
// @Success 200 {object} map[string]string "Account deleted"
// @Failure 400 {object} map[string]string "Invalid account ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 404 {object} map[string]string "Account not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /accounts/{id} [delete]
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	accountID := c.Param("id")
	accountObjID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	err = h.userService.DeleteAccount(userID.(string), accountObjID)
	if err != nil {
		if err.Error() == "account not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	metadata := map[string]interface{}{
		"user_id":    userID,
		"account_id": accountID,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "DeleteAccount", "User account deleted", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "Account deleted"})
}

// @Summary Edit user
// @Description Edit new user account via Telegram
// @Tags Users
// @Accept json
// @Produce json
// @Param user body models.UserAccount true "User account details"
// @Success 201 {object} map[string]interface{} "User created"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 409 {object} map[string]string "User already exists"
// @Failure 500 {object} map[string]string "Server error"
// @Router /users/edit [post]
func (h *UserHandler) EditUser(c *gin.Context) {
	var user models.UserAccount
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON error: %v", err)})
		return
	}

	existingUser, err := h.userService.GetUserByTelegramID(user.TelegramID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing user"})
		return
	}
	if existingUser == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this Telegram ID not exists"})
		return
	}

	user.ID = existingUser.ID

	if err := h.userService.EditUser(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to edit user"})
		return
	}

	metadata := map[string]interface{}{
		"username": user.Username,
		"user_id":  user.ID.Hex(),
	}
	if err := h.logService.LogAction(user.ID, "UserSignup", "User edited up via Telegram", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "User edited",
		"user_id": user.ID.Hex(),
		"user":    user,
	})
}

// @Summary User login
// @Description Validates a user via Telegram ID
// @Tags Users
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Telegram ID"
// @Success 200 {object} map[string]interface{} "User details"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 401 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	user, err := h.userService.GetUserByTelegramID(req.TelegramID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	metadata := map[string]interface{}{
		"user_id": user.ID.Hex(),
	}
	if err := h.logService.LogAction(user.ID, "UserLogin", "User logged in via Telegram", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "Login successful",
		"user":   user,
	})
}

// @Summary Get user by ID
// @Description Retrieves details of a user by their ID
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.UserAccount
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 404 {object} map[string]string "User not found"
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	user, err := h.userService.GetUserByTelegramID(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	metadata := map[string]interface{}{
		"id": id,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetUser", "User data retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, user)
}

// @Summary Get all users
// @Description Retrieves a list of all users
// @Tags Users
// @Produce json
// @Success 200 {array} models.UserAccount
// @Failure 500 {object} map[string]string "Server error"
// @Router /users [get]
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	metadata := map[string]interface{}{
		"count": len(users),
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetAllUsers", "Retrieved all users", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, users)
}

// @Summary Get current user
// @Description Retrieves the user's information using Telegram ID
// @Tags Users
// @Produce json
// @Param id path string true "Telegram ID of the user"
// @Success 200 {object} models.UserAccount
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /users/me/{id} [get]
func (h *UserHandler) GetMe(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
		return
	}

	user, err := h.userService.GetUserByTelegramID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	metadata := map[string]interface{}{
		"user_id": userID,
	}

	if err := h.logService.LogAction(primitive.NilObjectID, "GetMe", "Retrieved own profile", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, user)
}

// @Summary Transfer balance between accounts
// @Description Transfers balance between accounts (main, demo, or real) owned by the same user
// @Tags Users
// @Accept json
// @Produce json
// @Param transfer body TransferRequest true "Transfer details"
// @Success 200 {object} map[string]interface{} "Transfer successful"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Insufficient balance or invalid transfer"
// @Failure 404 {object} map[string]string "Account not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /accounts/transfer [post]
func (h *UserHandler) TransferBalance(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userStr := userID.(string)

	sourceObjID, err := primitive.ObjectIDFromHex(req.SourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	destObjID, err := primitive.ObjectIDFromHex(req.DestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid destination ID"})
		return
	}

	if sourceObjID == destObjID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Source and destination cannot be the same"})
		return
	}

	source, err := h.userService.GetUser(req.SourceID)
	if err != nil || source == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source account not found"})
		return
	}

	if source.TelegramID != userStr {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Source account does not belong to you"})
		return
	}

	dest, err := h.userService.GetUser(req.DestID)
	if err != nil || dest == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Destination account not found"})
		return
	}

	if dest.TelegramID != userStr {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Destination account does not belong to you"})
		return
	}

	// Validate account types
	if source.AccountType != "main" && source.AccountType != "demo" && source.AccountType != "real" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source account type"})
		return
	}

	if dest.AccountType != "main" && dest.AccountType != "demo" && dest.AccountType != "real" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid destination account type"})
		return
	}

	// Restrict transfers between demo and real accounts
	if (source.AccountType == "demo" && dest.AccountType == "real") ||
		(source.AccountType == "real" && dest.AccountType == "demo") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot transfer between demo and real accounts"})
		return
	}

	err = h.userService.TransferBalance(sourceObjID, destObjID, req.Amount, source.AccountType, dest.AccountType)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "insufficient balance"):
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient balance in source account"})
		case strings.Contains(err.Error(), "source account not found") || strings.Contains(err.Error(), "destination account not found"):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case strings.Contains(err.Error(), "invalid source account type") || strings.Contains(err.Error(), "invalid destination account type"):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Transfer failed: %v", err)})
		}
		return
	}

	updatedSource, err := h.userService.GetUser(req.SourceID)
	if err != nil || updatedSource == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated source account"})
		return
	}

	updatedDest, err := h.userService.GetUser(req.DestID)
	if err != nil || updatedDest == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated destination account"})
		return
	}

	var sourceBalance float64
	switch updatedSource.AccountType {
	case "main":
		sourceBalance = updatedSource.Balance
	case "demo":
		sourceBalance = updatedSource.DemoMT5Balance
	case "real":
		sourceBalance = updatedSource.RealMT5Balance
	}

	var destBalance float64
	switch updatedDest.AccountType {
	case "main":
		destBalance = updatedDest.Balance
	case "demo":
		destBalance = updatedDest.DemoMT5Balance
	case "real":
		destBalance = updatedDest.RealMT5Balance
	}

	metadata := map[string]interface{}{
		"source_id":   req.SourceID,
		"dest_id":     req.DestID,
		"amount":      req.Amount,
		"source_type": source.AccountType,
		"dest_type":   dest.AccountType,
	}
	if err := h.logService.LogAction(source.ID, "TransferBalance", "Transferred balance between accounts", c.ClientIP(), metadata); err != nil {
		log.Printf("Failed to log transfer action: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "Transfer successful",
		"source_balance": sourceBalance,
		"dest_balance":   destBalance,
	})
}
