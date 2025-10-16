package api

import (
	"log"
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LeaderRequestHandler struct {
	leaderRequestService service.LeaderRequestService
	logService           service.LogService
}

func NewLeaderRequestHandler(leaderRequestService service.LeaderRequestService, logService service.LogService) *LeaderRequestHandler {
	return &LeaderRequestHandler{leaderRequestService: leaderRequestService, logService: logService}
}

type CreateLeaderRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type ManageLeaderRequest struct {
	AdminReason string `json:"admin_reason" binding:"required"`
}

// @Summary Request to become a copy trade leader
// @Description Allows a user to submit a request to become a copy trade leader
// @Tags CopyTrading
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateLeaderRequest true "Leader request data"
// @Success 201 {object} map[string]string "Leader request created"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to create leader request"
// @Router /leader-requests [post]
func (h *LeaderRequestHandler) CreateLeaderRequest(c *gin.Context) {
	var req CreateLeaderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID := c.GetString("user_id")
	request, err := h.leaderRequestService.CreateLeaderRequest(userID, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"request_id": request.ID.Hex(),
		"user_id":    userID,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "CreateLeaderRequest", "Leader request created", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Leader request created", "request_id": request.ID.Hex()})
}

// @Summary Approve a leader request
// @Description Allows an admin to approve a leader request
// @Tags CopyTrading
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Request ID"
// @Param request body ManageLeaderRequest true "Admin reason"
// @Success 200 {object} map[string]string "Leader request approved"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to approve leader request"
// @Router /leader-requests/{id}/approve [post]
func (h *LeaderRequestHandler) ApproveLeaderRequest(c *gin.Context) {
	if !c.GetBool("is_admin") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin access required"})
		return
	}

	var req ManageLeaderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	requestID := c.Param("id")
	err := h.leaderRequestService.ApproveLeaderRequest(requestID, req.AdminReason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"request_id": requestID,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "ApproveLeaderRequest", "Leader request approved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "Leader request approved"})
}

// @Summary Deny a leader request
// @Description Allows an admin to deny a leader request
// @Tags CopyTrading
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Request ID"
// @Param request body ManageLeaderRequest true "Admin reason"
// @Success 200 {object} map[string]string "Leader request denied"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to deny leader request"
// @Router /leader-requests/{id}/deny [post]
func (h *LeaderRequestHandler) DenyLeaderRequest(c *gin.Context) {
	if !c.GetBool("is_admin") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin access required"})
		return
	}

	var req ManageLeaderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	requestID := c.Param("id")
	err := h.leaderRequestService.DenyLeaderRequest(requestID, req.AdminReason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"request_id": requestID,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "DenyLeaderRequest", "Leader request denied", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "Leader request denied"})
}

// @Summary Get pending leader requests
// @Description Retrieves all pending leader requests for admin review
// @Tags CopyTrading
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.LeaderRequest
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to retrieve leader requests"
// @Router /leader-requests [get]
func (h *LeaderRequestHandler) GetPendingLeaderRequests(c *gin.Context) {
	if !c.GetBool("is_admin") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin access required"})
		return
	}

	requests, err := h.leaderRequestService.GetPendingLeaderRequests()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"count": len(requests),
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetPendingLeaderRequests", "Pending leader requests retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, requests)
}

// @Summary Get approved copy trade leaders
// @Description Retrieves a list of approved copy trade leaders
// @Tags CopyTrading
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.User
// @Failure 500 {object} map[string]string "Failed to retrieve leaders"
// @Router /copy-trade-leaders [get]
func (h *LeaderRequestHandler) GetApprovedLeaders(c *gin.Context) {
	leaders, err := h.leaderRequestService.GetApprovedLeaders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"count": len(leaders),
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetApprovedLeaders", "Approved leaders retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, leaders)
}
