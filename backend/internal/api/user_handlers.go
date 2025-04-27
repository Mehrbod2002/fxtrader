package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserHandler struct {
	userService service.UserService
	logService  service.LogService
}

func NewUserHandler(userService service.UserService, logService service.LogService) *UserHandler {
	return &UserHandler{userService: userService, logService: logService}
}

func (h *UserHandler) SignupUser(c *gin.Context) {
	var user models.UserAccount
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

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
