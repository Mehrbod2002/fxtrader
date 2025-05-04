package api

import (
	"net/http"

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
// @Description Retrieves a list of all system logs (admin only)
// @Tags Logs
// @Produce json
// @Security BasicAuth
// @Success 200 {array} models.LogEntry
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to retrieve logs"
// @Router /admin/logs [get]
func (h *LogHandler) GetAllLogs(c *gin.Context) {
	logs, err := h.logService.GetAllLogs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

// @Summary Get logs by user ID
// @Description Retrieves logs associated with a specific user ID (admin only)
// @Tags Logs
// @Produce json
// @Security BasicAuth
// @Param user_id path string true "User ID"
// @Success 200 {array} models.LogEntry
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /admin/logs/user/{user_id} [get]
func (h *LogHandler) GetLogsByUser(c *gin.Context) {
	userID := c.Param("user_id")
	logs, err := h.logService.GetLogsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	c.JSON(http.StatusOK, logs)
}
