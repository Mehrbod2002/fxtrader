package service

import (
	"errors"
	"math"

	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CopyTradeService interface {
	CreateSubscription(followerID, leaderID string, allocatedAmount float64, accountType string) (*models.CopyTradeSubscription, error)
	GetSubscription(id string) (*models.CopyTradeSubscription, error)
	GetSubscriptionsByFollowerID(followerID string) ([]*models.CopyTradeSubscription, error)
	MirrorTrade(leaderTrade *models.TradeHistory, accountType string) error
	SetTradeService(tradeService TradeService)
}

type copyTradeService struct {
	copyTradeRepo repository.CopyTradeRepository
	tradeService  TradeService
	userService   UserService
	logService    LogService
}

func (s *copyTradeService) SetTradeService(tradeService TradeService) {
	s.tradeService = tradeService
}

func NewCopyTradeService(copyTradeRepo repository.CopyTradeRepository, tradeService TradeService, userService UserService, logService LogService) CopyTradeService {
	return &copyTradeService{
		copyTradeRepo: copyTradeRepo,
		tradeService:  tradeService,
		userService:   userService,
		logService:    logService,
	}
}

func (s *copyTradeService) CreateSubscription(followerID, leaderID string, allocatedAmount float64, accountType string) (*models.CopyTradeSubscription, error) {
	if followerID == leaderID {
		return nil, errors.New("cannot follow yourself")
	}
	if allocatedAmount <= 0 {
		return nil, errors.New("allocated amount must be positive")
	}

	follower, err := s.userService.GetUser(followerID)
	if err != nil || follower == nil {
		return nil, errors.New("follower not found")
	}
	leader, err := s.userService.GetUser(leaderID)
	if err != nil || leader == nil {
		return nil, errors.New("leader not found")
	}

	followerBalance, err := s.tradeService.RequestBalance(followerID, accountType)
	if err != nil {
		return nil, errors.New("failed to fetch follower balance")
	}
	if followerBalance < allocatedAmount {
		return nil, errors.New("insufficient balance")
	}

	subscription := &models.CopyTradeSubscription{
		FollowerID:      followerID,
		LeaderID:        leaderID,
		AllocatedAmount: allocatedAmount,
		Status:          "ACTIVE",
	}

	err = s.copyTradeRepo.SaveSubscription(subscription)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"subscription_id":  subscription.ID.Hex(),
		"follower_id":      followerID,
		"leader_id":        leaderID,
		"allocated_amount": allocatedAmount,
	}
	s.logService.LogAction(primitive.ObjectID{}, "CreateCopySubscription", "Copy trade subscription created", "", metadata)

	return subscription, nil
}

func (s *copyTradeService) GetSubscription(id string) (*models.CopyTradeSubscription, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid subscription ID")
	}
	return s.copyTradeRepo.GetSubscriptionByID(objID)
}

func (s *copyTradeService) GetSubscriptionsByFollowerID(followerID string) ([]*models.CopyTradeSubscription, error) {
	return s.copyTradeRepo.GetSubscriptionsByFollowerID(followerID)
}

func (s *copyTradeService) MirrorTrade(leaderTrade *models.TradeHistory, accountType string) error {
	subscriptions, err := s.copyTradeRepo.GetActiveSubscriptionsByLeaderID(leaderTrade.UserID.Hex())
	if err != nil {
		return err
	}

	leaderBalance, err := s.tradeService.RequestBalance(leaderTrade.UserID.Hex(), accountType)
	if err != nil {
		return errors.New("failed to fetch leader balance")
	}
	if leaderBalance <= 0 {
		return errors.New("leader balance is zero")
	}

	volumeRatio := leaderTrade.Volume / leaderBalance

	for _, sub := range subscriptions {
		followerBalance, err := s.tradeService.RequestBalance(sub.FollowerID, accountType)
		if err != nil {
			continue
		}

		followerVolume := math.Min(sub.AllocatedAmount, followerBalance) * volumeRatio
		followerTrade, err := s.tradeService.PlaceTrade(
			sub.FollowerID,
			leaderTrade.Symbol,
			accountType,
			leaderTrade.TradeType,
			leaderTrade.OrderType,
			leaderTrade.Leverage,
			followerVolume,
			leaderTrade.EntryPrice,
			leaderTrade.StopLoss,
			leaderTrade.TakeProfit,
			leaderTrade.Expiration,
		)
		if err != nil {
			continue
		}

		copyTrade := &models.CopyTrade{
			SubscriptionID:  sub.ID,
			LeaderTradeID:   leaderTrade.ID,
			FollowerTradeID: followerTrade.ID,
		}
		err = s.copyTradeRepo.SaveCopyTrade(copyTrade)
		if err != nil {
			continue
		}

		metadata := map[string]interface{}{
			"copy_trade_id":     copyTrade.ID.Hex(),
			"subscription_id":   sub.ID.Hex(),
			"leader_trade_id":   leaderTrade.ID.Hex(),
			"follower_trade_id": followerTrade.ID.Hex(),
			"follower_volume":   followerVolume,
		}
		s.logService.LogAction(primitive.ObjectID{}, "MirrorTrade", "Trade mirrored for follower", "", metadata)
	}

	return nil
}
