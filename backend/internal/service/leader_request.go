package service

import (
	"errors"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LeaderRequestService interface {
	CreateLeaderRequest(userID, reason string) (*models.LeaderRequest, error)
	ApproveLeaderRequest(requestID string, adminReason string) error
	DenyLeaderRequest(requestID string, adminReason string) error
	GetPendingLeaderRequests() ([]*models.LeaderRequest, error)
	GetApprovedLeaders() ([]*models.UserAccount, error)
}

type leaderRequestService struct {
	leaderRequestRepo repository.LeaderRequestRepository
	userService       UserService
	logService        LogService
}

func NewLeaderRequestService(
	leaderRequestRepo repository.LeaderRequestRepository,
	userService UserService,
	logService LogService,
) LeaderRequestService {
	return &leaderRequestService{
		leaderRequestRepo: leaderRequestRepo,
		userService:       userService,
		logService:        logService,
	}
}

func (s *leaderRequestService) CreateLeaderRequest(userID, reason string) (*models.LeaderRequest, error) {
	user, err := s.userService.GetUser(userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if user.IsCopyTradeLeader {
		return nil, errors.New("user is already a copy trade leader")
	}

	request := &models.LeaderRequest{
		UserID: userID,
		Reason: reason,
		Status: "PENDING",
	}
	err = s.leaderRequestRepo.SaveLeaderRequest(request)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"request_id": request.ID.Hex(),
		"user_id":    userID,
	}
	s.logService.LogAction(primitive.ObjectID{}, "CreateLeaderRequest", "Leader request created", "", metadata)
	return request, nil
}

func (s *leaderRequestService) ApproveLeaderRequest(requestID, adminReason string) error {
	objID, err := primitive.ObjectIDFromHex(requestID)
	if err != nil {
		return errors.New("invalid request ID")
	}

	request, err := s.leaderRequestRepo.GetLeaderRequestByID(objID)
	if err != nil {
		return err
	}
	if request == nil {
		return errors.New("request not found")
	}
	if request.Status != "PENDING" {
		return errors.New("request is not pending")
	}

	request.Status = "APPROVED"
	request.AdminReason = adminReason
	err = s.leaderRequestRepo.UpdateLeaderRequest(request)
	if err != nil {
		return err
	}

	user, err := s.userService.GetUser(request.UserID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	user.IsCopyTradeLeader = true
	err = s.userService.UpdateUser(user)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"request_id":   requestID,
		"user_id":      request.UserID,
		"admin_reason": adminReason,
	}
	s.logService.LogAction(primitive.ObjectID{}, "ApproveLeaderRequest", "Leader request approved", "", metadata)
	return nil
}

func (s *leaderRequestService) DenyLeaderRequest(requestID, adminReason string) error {
	objID, err := primitive.ObjectIDFromHex(requestID)
	if err != nil {
		return errors.New("invalid request ID")
	}

	request, err := s.leaderRequestRepo.GetLeaderRequestByID(objID)
	if err != nil {
		return err
	}
	if request == nil {
		return errors.New("request not found")
	}
	if request.Status != "PENDING" {
		return errors.New("request is not pending")
	}

	request.Status = "DENIED"
	request.AdminReason = adminReason
	err = s.leaderRequestRepo.UpdateLeaderRequest(request)
	if err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"request_id":   requestID,
		"user_id":      request.UserID,
		"admin_reason": adminReason,
	}
	s.logService.LogAction(primitive.ObjectID{}, "DenyLeaderRequest", "Leader request denied", "", metadata)
	return nil
}

func (s *leaderRequestService) GetPendingLeaderRequests() ([]*models.LeaderRequest, error) {
	return s.leaderRequestRepo.GetPendingLeaderRequests()
}

func (s *leaderRequestService) GetApprovedLeaders() ([]*models.UserAccount, error) {
	return s.userService.GetUsersByLeaderStatus(true)
}
