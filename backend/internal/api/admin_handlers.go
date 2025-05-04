package api

import (
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/middleware"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"github.com/mehrbod2002/fxtrader/internal/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	adminRepo repository.AdminRepository
	cfg       *config.Config
}

func NewAdminHandler(adminRepo repository.AdminRepository, cfg *config.Config) *AdminHandler {
	return &AdminHandler{adminRepo: adminRepo, cfg: cfg}
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
