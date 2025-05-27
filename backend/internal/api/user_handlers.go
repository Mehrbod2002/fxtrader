package api

import (
	"log"
	"net/http"
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
		AccountType:      "user",
		AccountTypes:     []string{"REAL", "DEMO"},
		RegistrationDate: time.Now().Format(time.RFC3339),
	}

	if user.Username == "" {
		user.Username = "user_" + user.TelegramID
	}

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
