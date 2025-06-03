package service

import (
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LogService interface {
	LogAction(userID primitive.ObjectID, action, description, ipAddress string, metadata map[string]interface{}) error
	GetAllLogs(page, limit int) ([]*models.LogEntry, error)
	GetLogsByUserID(userID string, page, limit int) ([]*models.LogEntry, error)
}

type logService struct {
	logRepo repository.LogRepository
}

func NewLogService(logRepo repository.LogRepository) LogService {
	return &logService{logRepo: logRepo}
}

func (s *logService) LogAction(userID primitive.ObjectID, action, description, ipAddress string, metadata map[string]interface{}) error {
	logEntry := &models.LogEntry{
		UserID:      userID,
		Action:      action,
		Description: description,
		IPAddress:   ipAddress,
		Metadata:    metadata,
	}
	return s.logRepo.SaveLog(logEntry)
}

func (s *logService) GetAllLogs(page, limit int) ([]*models.LogEntry, error) {
	return s.logRepo.GetAllLogs(page, limit)
}

func (s *logService) GetLogsByUserID(userID string, page, limit int) ([]*models.LogEntry, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	return s.logRepo.GetLogsByUserID(objID, page, limit)
}
