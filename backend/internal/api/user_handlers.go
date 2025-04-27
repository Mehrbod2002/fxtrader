package api

import (
	"fxtrader/internal/config"
	"fxtrader/internal/middleware"
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	userService service.UserService
	logService  service.LogService
	cfg         *config.Config
}

func NewUserHandler(userService service.UserService, logService service.LogService, cfg *config.Config) *UserHandler {
	return &UserHandler{userService: userService, logService: logService, cfg: cfg}
}

// @Summary User login
// @Description Authenticates a user and returns a JWT token
// @Tags Users
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "User credentials"
// @Success 200 {object} map[string]string "Token"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 401 {object} map[string]string "Invalid credentials"
// @Router /users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	user, err := h.userService.GetUserByTelegramID(req.TelegramID)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := middleware.GenerateJWT(user.ID.Hex(), h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	metadata := map[string]interface{}{
		"user_id": user.ID.Hex(),
	}
	h.logService.LogAction(user.ID, "UserLogin", "User logged in", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *UserHandler) SignupUser(c *gin.Context) {
	var user models.UserAccount
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = string(hashedPassword)

	if err := h.userService.SignupUser(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	metadata := map[string]interface{}{
		"username": user.Username,
		"user_id":  user.ID.Hex(),
	}
	h.logService.LogAction(user.ID, "UserSignup", "User signed up", c.ClientIP(), metadata)

	c.JSON(http.StatusCreated, gin.H{"status": "User created", "user_id": user.ID.Hex()})
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	user, err := h.userService.GetUser(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	metadata := map[string]interface{}{
		"user_id": id,
	}
	h.logService.LogAction(primitive.ObjectID{}, "GetUser", "User data retrieved", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, user)
}

type LoginRequest struct {
	TelegramID string `json:"telegram_id" binding:"required"`
	Password   string `json:"password" binding:"required"`
}
