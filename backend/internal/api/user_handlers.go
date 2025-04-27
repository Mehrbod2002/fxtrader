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

// SignupUser creates a new user
// @Summary Create a new user
// @Description Registers a new user account
// @Tags Users
// @Accept json
// @Produce json
// @Param user body models.UserAccount true "User account data"
// @Success 201 {object} map[string]string "User created"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 500 {object} map[string]string "Failed to create user"
// @Router /users/signup [post]
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

// GetUser retrieves a user by ID
// @Summary Get user by ID
// @Description Retrieves user account details by user ID
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.UserAccount
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 404 {object} map[string]string "User not found"
// @Router /users/{id} [get]
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
