package api

import (
	"log"
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CopyTradeHandler struct {
	copyTradeService service.CopyTradeService
	logService       service.LogService
}

func NewCopyTradeHandler(copyTradeService service.CopyTradeService, logService service.LogService) *CopyTradeHandler {
	return &CopyTradeHandler{copyTradeService: copyTradeService, logService: logService}
}

// @Summary Create a copy trade subscription
// @Description Allows a user to follow a trader and allocate funds for copy trading
// @Tags CopyTrading
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param subscription body CopyTradeRequest true "Subscription data"
// @Success 201 {object} map[string]string "Subscription created"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to create subscription"
// @Router /copy-trades [post]
func (h *CopyTradeHandler) CreateSubscription(c *gin.Context) {
	var req CopyTradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	followerID := c.GetString("user_id")
	subscription, err := h.copyTradeService.CreateSubscription(followerID, req.LeaderID, req.AllocatedAmount, req.AccountType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"subscription_id": subscription.ID.Hex(),
		"follower_id":     followerID,
		"leader_id":       req.LeaderID,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "CreateCopySubscription", "Copy trade subscription created", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Subscription created", "subscription_id": subscription.ID.Hex()})
}

// @Summary Get user copy trade subscriptions
// @Description Retrieves all copy trade subscriptions for the authenticated user
// @Tags CopyTrading
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.CopyTradeSubscription
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to retrieve subscriptions"
// @Router /copy-trades-all [get]
func (h *CopyTradeHandler) GetAllUserSubscriptions(c *gin.Context) {
	subscriptions, err := h.copyTradeService.GetAllSubscriptions()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}

// @Summary Get user copy trade subscriptions
// @Description Retrieves all copy trade subscriptions for the authenticated user
// @Tags CopyTrading
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.CopyTradeSubscription
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to retrieve subscriptions"
// @Router /copy-trades [get]
func (h *CopyTradeHandler) GetUserSubscriptions(c *gin.Context) {
	followerID := c.GetString("user_id")
	subscriptions, err := h.copyTradeService.GetSubscriptionsByFollowerID(followerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"follower_id": followerID,
		"count":       len(subscriptions),
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetCopySubscriptions", "User copy subscriptions retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, subscriptions)
}

// @Summary Get copy trade subscription by ID
// @Description Retrieves details of a specific copy trade subscription
// @Tags CopyTrading
// @Produce json
// @Security BearerAuth
// @Param id path string true "Subscription ID"
// @Success 200 {object} models.CopyTradeSubscription
// @Failure 400 {object} map[string]string "Invalid subscription ID"
// @Failure 404 {object} map[string]string "Subscription not found"
// @Router /copy-trades/{id} [get]
func (h *CopyTradeHandler) GetSubscription(c *gin.Context) {
	subscriptionID := c.Param("id")
	subscription, err := h.copyTradeService.GetSubscription(subscriptionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID"})
		return
	}
	if subscription == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
		return
	}

	metadata := map[string]interface{}{
		"subscription_id": subscriptionID,
	}
	if err := h.logService.LogAction(primitive.ObjectID{}, "GetCopySubscription", "Copy subscription data retrieved", c.ClientIP(), metadata); err != nil {
		log.Printf("error: %v", err)
	}

	c.JSON(http.StatusOK, subscription)
}

type CopyTradeRequest struct {
	LeaderID        string  `json:"leader_id" binding:"required"`
	AccountType     string  `json:"account_type" binding:"required"`
	AllocatedAmount float64 `json:"allocated_amount" binding:"required,gt=0"`
}
