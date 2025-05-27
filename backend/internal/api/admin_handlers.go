package api

import (
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/middleware"
	"github.com/mehrbod2002/fxtrader/internal/repository"
	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/mehrbod2002/fxtrader/internal/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	adminRepo   repository.AdminRepository
	cfg         *config.Config
	userService service.UserService
}

func NewAdminHandler(adminRepo repository.AdminRepository, cfg *config.Config, userService service.UserService) *AdminHandler {
	return &AdminHandler{
		adminRepo:   adminRepo,
		cfg:         cfg,
		userService: userService,
	}
}

type UserActivationRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	IsActive bool   `json:"is_active"`
}

// @Summary Activate or deactivate a user
// @Description Allows for an admin
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param activation body UserActivationRequest true "User activation data"
// @Success 200 {object} map[string]string "User status updated"
// @Failure 400 {object} map[string]string "Invalid JSON or user ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Failed to update user status"
// @Router /admin/users/activation [put]
func (h *AdminHandler) UpdateUserActivation(c *gin.Context) {
	var req UserActivationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	user, err := h.userService.GetUser(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.IsActive = req.IsActive
	err = h.userService.SignupUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user status"})
		return
	}

	status := "deactivated"
	if req.IsActive {
		status = "activated"
	}

	c.JSON(http.StatusOK, gin.H{"status": "User " + status})
}

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// @Summary Admin login
// @Description Authenticates an admin user and returns a JWT token
// @Tags Admin
// @Accept json
// @Produce json
// @Param credentials body AdminLoginRequest true "Admin credentials"
// @Success 200 {object} map[string]string "JWT token"
// @Failure 400 {object} map[string]string "Invalid JSON"
// @Failure 401 {object} map[string]string "Invalid credentials"
// @Failure 500 {object} map[string]string "Server error"
// @Router /admin/login [post]
func (h *AdminHandler) AdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	admin, err := h.adminRepo.GetAdminByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve admin"})
		return
	}
	if admin == nil || admin.AccountType != "admin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := middleware.GenerateAdminJWT(admin.ID.Hex(), h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "Login successful",
		"token":  token,
	})
}
