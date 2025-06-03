package api

import (
	"net/http"
	"strconv"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
)

type LogHandler struct {
	logService service.LogService
}

func NewLogHandler(logService service.LogService) *LogHandler {
	return &LogHandler{logService: logService}
}

// @Summary Get all logs
// @Description Retrieves a paginated list of all system logs (admin only)
// @Tags Logs
// @Produce json
// @Security BasicAuth
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Number of logs per page (default 100)"
// @Success 200 {array} models.LogEntry
// @Failure 400 {object} map[string]string "Invalid pagination parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to retrieve logs"
// @Router /admin/logs [get]
func (h *LogHandler) GetAllLogs(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit value"})
		return
	}

	logs, err := h.logService.GetAllLogs(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

// @Summary Get logs by user ID
// @Description Retrieves a paginated list of logs associated with a specific user ID (admin only)
// @Tags Logs
// @Produce json
// @Security BasicAuth
// @Param user_id path string true "User ID"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Number of logs per page (default 100)"
// @Success 200 {array} models.LogEntry
// @Failure 400 {object} map[string]string "Invalid user ID or pagination parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /admin/logs/user/{user_id} [get]
func (h *LogHandler) GetLogsByUser(c *gin.Context) {
	userID := c.Param("user_id")
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit value"})
		return
	}

	logs, err := h.logService.GetLogsByUserID(userID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	c.JSON(http.StatusOK, logs)
}
