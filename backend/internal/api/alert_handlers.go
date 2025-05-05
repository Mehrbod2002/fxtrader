package api

import (
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlertHandler struct {
	alertService service.AlertService
	logService   service.LogService
}

func NewAlertHandler(alertService service.AlertService, logService service.LogService) *AlertHandler {
	return &AlertHandler{alertService: alertService, logService: logService}
}

// @Summary Create a new alert
// @Description Allows a user to create a price or time-based alert
// @Tags Alerts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param alert body AlertRequest true "Alert data"
// @Success 201 {object} map[string]string "Alert created"
// @Failure 400 {object} map[string]string "Invalid JSON or parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to create alert"
// @Router /alerts [post]
func (h *AlertHandler) CreateAlert(c *gin.Context) {
	var req AlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	userID := c.GetString("user_id")
	alert := &models.Alert{
		SymbolName:         req.SymbolName,
		AlertType:          req.AlertType,
		Condition:          req.Condition,
		NotificationMethod: req.NotificationMethod,
	}

	if err := h.alertService.CreateAlert(userID, alert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"alert_id":    alert.ID.Hex(),
		"symbol_name": alert.SymbolName,
		"alert_type":  alert.AlertType,
	}
	h.logService.LogAction(primitive.ObjectID{}, "CreateAlert", "Alert created", c.ClientIP(), metadata)

	c.JSON(http.StatusCreated, gin.H{"status": "Alert created", "alert_id": alert.ID.Hex()})
}

// @Summary Get user alerts
// @Description Retrieves all alerts for the authenticated user
// @Tags Alerts
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Alert
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to retrieve alerts"
// @Router /alerts [get]
func (h *AlertHandler) GetUserAlerts(c *gin.Context) {
	userID := c.GetString("user_id")
	alerts, err := h.alertService.GetAlertsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"user_id": userID,
		"count":   len(alerts),
	}
	h.logService.LogAction(primitive.ObjectID{}, "GetUserAlerts", "User alerts retrieved", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, alerts)
}

// @Summary Get alert by ID
// @Description Retrieves details of a specific alert
// @Tags Alerts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Alert ID"
// @Success 200 {object} models.Alert
// @Failure 400 {object} map[string]string "Invalid alert ID"
// @Failure 404 {object} map[string]string "Alert not found"
// @Router /alerts/{id} [get]
func (h *AlertHandler) GetAlert(c *gin.Context) {
	alertID := c.Param("id")
	alert, err := h.alertService.GetAlert(alertID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alert ID"})
		return
	}
	if alert == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
		return
	}

	metadata := map[string]interface{}{
		"alert_id": alertID,
	}
	h.logService.LogAction(primitive.ObjectID{}, "GetAlert", "Alert data retrieved", c.ClientIP(), metadata)

	c.JSON(http.StatusOK, alert)
}

type AlertRequest struct {
	SymbolName         string                `json:"symbol_name" binding:"required"`
	AlertType          models.AlertType      `json:"alert_type" binding:"required,oneof=PRICE TIME"`
	Condition          models.AlertCondition `json:"condition" binding:"required"`
	NotificationMethod string                `json:"notification_method" binding:"required,oneof=SMS EMAIL"`
}
