package api

import (
	"net/http"
	"strconv"

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

type UserReferralResponse struct {
	UserID        string   `json:"user_id"`
	Username      string   `json:"username"`
	ReferralCode  string   `json:"referral_code"`
	ReferredBy    string   `json:"referred_by"`
	ReferredUsers []string `json:"referred_users"`
}

type PaginatedReferralsResponse struct {
	Users      []UserReferralResponse `json:"users"`
	Total      int64                  `json:"total"`
	Page       int64                  `json:"page"`
	Limit      int64                  `json:"limit"`
	TotalPages int64                  `json:"total_pages"`
}

// @Summary Get user's referral information
// @Description Retrieves the referral details for the authenticated user (who referred them and who they referred)
// @Tags User
// @Accept json
// @Produce json
// @Security X-Telegram-ID
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} UserReferralResponse "User referral data"
// @Failure 400 {object} map[string]string "Invalid query parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/v1/referrals [get]
func (h *AdminHandler) GetUserReferrals(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	user, err := h.userService.GetUser(userIDStr)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	page, err := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	limit, err := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 64)
	if err != nil || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}

	referredUsers, _, err := h.userService.GetUsersReferredBy(user.ReferralCode, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch referred users"})
		return
	}

	referredUserIDs := make([]string, len(referredUsers))
	for i, u := range referredUsers {
		referredUserIDs[i] = u.ID.Hex()
	}

	response := UserReferralResponse{
		UserID:        user.ID.Hex(),
		Username:      user.Username,
		ReferralCode:  user.ReferralCode,
		ReferredBy:    user.ReferredBy.String(),
		ReferredUsers: referredUserIDs,
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get all referrals (Admin)
// @Description Retrieves all users' referral data with pagination (admin only)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} PaginatedReferralsResponse "Paginated referral data"
// @Failure 400 {object} map[string]string "Invalid query parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/v1/admin/referrals [get]
func (h *AdminHandler) GetAllReferrals(c *gin.Context) {
	_, exists := c.Get("is_admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin access required"})
		return
	}

	page, err := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	limit, err := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 64)
	if err != nil || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}

	users, total, err := h.userService.GetAllReferrals(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch referrals"})
		return
	}

	responseUsers := make([]UserReferralResponse, len(users))
	for i, user := range users {
		referredUsers, _, err := h.userService.GetUsersReferredBy(user.ReferralCode, 1, 1000)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch referred users"})
			return
		}

		referredUserIDs := make([]string, len(referredUsers))
		for j, u := range referredUsers {
			referredUserIDs[j] = u.ID.Hex()
		}

		responseUsers[i] = UserReferralResponse{
			UserID:        user.ID.Hex(),
			Username:      user.Username,
			ReferralCode:  user.ReferralCode,
			ReferredBy:    user.ReferredBy.Hex(),
			ReferredUsers: referredUserIDs,
		}
	}

	response := PaginatedReferralsResponse{
		Users:      responseUsers,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: (total + limit - 1) / limit,
	}

	c.JSON(http.StatusOK, response)
}
