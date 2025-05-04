package service

import (
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RuleService interface {
	CreateRule(rule *models.Rule) error
	GetRule(id string) (*models.Rule, error)
	GetAllRules() ([]*models.Rule, error)
	UpdateRule(id string, rule *models.Rule) error
	DeleteRule(id string) error
}

type ruleService struct {
	ruleRepo repository.RuleRepository
}

func NewRuleService(ruleRepo repository.RuleRepository) RuleService {
	return &ruleService{ruleRepo: ruleRepo}
}

func (s *ruleService) CreateRule(rule *models.Rule) error {
	return s.ruleRepo.SaveRule(rule)
}

func (s *ruleService) GetRule(id string) (*models.Rule, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.ruleRepo.GetRuleByID(objID)
}

func (s *ruleService) GetAllRules() ([]*models.Rule, error) {
	return s.ruleRepo.GetAllRules()
}

func (s *ruleService) UpdateRule(id string, rule *models.Rule) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.ruleRepo.UpdateRule(objID, rule)
}

func (s *ruleService) DeleteRule(id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.ruleRepo.DeleteRule(objID)
}
