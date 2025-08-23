package service

import (
	"errors"
	"math"

	"github.com/mehrbod2002/fxtrader/interfaces"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CopyTradeService interface {
	CreateSubscription(followerID, leaderID string, allocatedAmount float64, accountType string) (*models.CopyTradeSubscription, error)
	GetSubscription(id string) (*models.CopyTradeSubscription, error)
	GetSubscriptionsByFollowerID(followerID string) ([]*models.CopyTradeSubscription, error)
	GetAllSubscriptions() ([]*models.CopyTradeSubscription, error)
	MirrorTrade(leaderTrade *models.TradeHistory, accountType string) error
	SetTradeService(tradeService interfaces.TradeService)
}

type copyTradeService struct {
	copyTradeRepo  repository.CopyTradeRepository
	tradeService   interfaces.TradeService
	userService    UserService
	accountService AccountService
	logService     LogService
}

func (s *copyTradeService) SetTradeService(tradeService interfaces.TradeService) {
	s.tradeService = tradeService
}

func NewCopyTradeService(copyTradeRepo repository.CopyTradeRepository, tradeService interfaces.TradeService, userService UserService, accountService AccountService, logService LogService) CopyTradeService {
	return &copyTradeService{
		copyTradeRepo:  copyTradeRepo,
		tradeService:   tradeService,
		userService:    userService,
		accountService: accountService,
		logService:     logService,
	}
}

func (s *copyTradeService) CreateSubscription(followerID, leaderID string, allocatedAmount float64, accountType string) (*models.CopyTradeSubscription, error) {
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
	if !leader.IsCopyTradeLeader {
		return nil, errors.New("user is not an approved copy trade leader")
	}

	accounts, err := s.accountService.GetAccountsByUserID(followerID)
	if err != nil {
		return nil, errors.New("failed to fetch follower accounts")
	}
	var followerAccount *models.Account
	for _, acc := range accounts {
		if acc.AccountType == accountType {
			followerAccount = acc
			break
		}
	}
	if followerAccount == nil {
		return nil, errors.New("follower does not have account of type " + accountType)
	}

	followerBalance, err := s.tradeService.RequestBalance(followerID, followerAccount.ID.Hex(), accountType)
	if err != nil {
		return nil, errors.New("failed to fetch follower balance")
	}
	if followerBalance < allocatedAmount {
		return nil, errors.New("insufficient balance")
	}

	subscription := &models.CopyTradeSubscription{
		FollowerID:         followerID,
		LeaderID:           leaderID,
		FollowerIDTelegram: follower.TelegramID,
		LeaderIDTelegram:   leader.TelegramID,
		AllocatedAmount:    allocatedAmount,
		AccountType:        accountType,
		Status:             "ACTIVE",
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
	if err := s.logService.LogAction(primitive.ObjectID{}, "CreateCopySubscription", "Copy trade subscription created", "", metadata); err != nil {
		return nil, err
	}

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

func (s *copyTradeService) GetAllSubscriptions() ([]*models.CopyTradeSubscription, error) {
	return s.copyTradeRepo.GetAllSubscriptions()
}

func (s *copyTradeService) MirrorTrade(leaderTrade *models.TradeHistory, accountType string) error {
	subscriptions, err := s.copyTradeRepo.GetActiveSubscriptionsByLeaderID(leaderTrade.UserID.Hex())
	if err != nil {
		return err
	}

	leaderAccountID := leaderTrade.AccountID.Hex()
	leaderBalance, err := s.tradeService.RequestBalance(leaderTrade.UserID.Hex(), leaderAccountID, accountType)
	if err != nil {
		return errors.New("failed to fetch leader balance")
	}
	if leaderBalance <= 0 {
		return errors.New("leader balance is zero")
	}

	volumeRatio := leaderTrade.Volume / leaderBalance

	for _, sub := range subscriptions {
		if sub.AccountType != accountType {
			continue
		}

		accounts, err := s.accountService.GetAccountsByUserID(sub.FollowerID)
		if err != nil {
			continue
		}
		var followerAccount *models.Account
		for _, acc := range accounts {
			if acc.AccountType == accountType {
				followerAccount = acc
				break
			}
		}
		if followerAccount == nil {
			continue
		}

		followerBalance, err := s.tradeService.RequestBalance(sub.FollowerID, followerAccount.ID.Hex(), accountType)
		if err != nil {
			continue
		}

		followerVolume := math.Min(sub.AllocatedAmount, followerBalance) * volumeRatio
		followerTrade, _, err := s.tradeService.PlaceTrade(
			sub.FollowerID,
			followerAccount.ID.Hex(),
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
		if err := s.logService.LogAction(primitive.ObjectID{}, "MirrorTrade", "Trade mirrored for follower", "", metadata); err != nil {
			return nil
		}
	}

	return nil
}
