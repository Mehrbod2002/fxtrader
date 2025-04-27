package api

import (
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type LogHandler struct {
	logService service.LogService
}

func NewLogHandler(logService service.LogService) *LogHandler {
	return &LogHandler{logService: logService}
}

func (h *LogHandler) GetAllLogs(c *gin.Context) {
	logs, err := h.logService.GetAllLogs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (h *LogHandler) GetLogsByUser(c *gin.Context) {
	userID := c.Param("user_id")
	logs, err := h.logService.GetLogsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	c.JSON(http.StatusOK, logs)
}
