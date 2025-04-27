package repository

import (
	"context"
	"fxtrader/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SymbolRepository interface {
	SaveSymbol(symbol *models.Symbol) error
	GetSymbolByID(id primitive.ObjectID) (*models.Symbol, error)
	GetAllSymbols() ([]*models.Symbol, error)
	UpdateSymbol(id primitive.ObjectID, symbol *models.Symbol) error
	DeleteSymbol(id primitive.ObjectID) error
}

type MongoSymbolRepository struct {
	collection *mongo.Collection
}

func NewSymbolRepository(client *mongo.Client, dbName, collectionName string) SymbolRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoSymbolRepository{collection: collection}
}

func (r *MongoSymbolRepository) SaveSymbol(symbol *models.Symbol) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbol.ID = primitive.NewObjectID()
	symbol.CreatedAt = time.Now()
	symbol.UpdatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, symbol)
	return err
}

func (r *MongoSymbolRepository) GetSymbolByID(id primitive.ObjectID) (*models.Symbol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var symbol models.Symbol
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&symbol)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &symbol, err
}

func (r *MongoSymbolRepository) GetAllSymbols() ([]*models.Symbol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var symbols []*models.Symbol
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}

func (r *MongoSymbolRepository) UpdateSymbol(id primitive.ObjectID, symbol *models.Symbol) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbol.UpdatedAt = time.Now()
	update := bson.M{"$set": symbol}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *MongoSymbolRepository) DeleteSymbol(id primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
