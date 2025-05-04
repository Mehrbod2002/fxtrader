package service

import (
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SymbolService interface {
	CreateSymbol(symbol *models.Symbol) error
	GetSymbol(id string) (*models.Symbol, error)
	GetAllSymbols() ([]*models.Symbol, error)
	UpdateSymbol(id string, symbol *models.Symbol) error
	DeleteSymbol(id string) error
}

type symbolService struct {
	symbolRepo repository.SymbolRepository
}

func NewSymbolService(symbolRepo repository.SymbolRepository) SymbolService {
	return &symbolService{symbolRepo: symbolRepo}
}

func (s *symbolService) CreateSymbol(symbol *models.Symbol) error {
	return s.symbolRepo.SaveSymbol(symbol)
}

func (s *symbolService) GetSymbol(id string) (*models.Symbol, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.symbolRepo.GetSymbolByID(objID)
}

func (s *symbolService) GetAllSymbols() ([]*models.Symbol, error) {
	return s.symbolRepo.GetAllSymbols()
}

func (s *symbolService) UpdateSymbol(id string, symbol *models.Symbol) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.symbolRepo.UpdateSymbol(objID, symbol)
}

func (s *symbolService) DeleteSymbol(id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.symbolRepo.DeleteSymbol(objID)
}
