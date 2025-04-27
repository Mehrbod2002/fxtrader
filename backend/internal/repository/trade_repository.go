package repository

import (
	"context"
	"fxtrader/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TradeRepository interface {
	SaveTrade(trade *models.TradeHistory) error
	GetTradeByID(id primitive.ObjectID) (*models.TradeHistory, error)
	GetTradesByUserID(userID primitive.ObjectID) ([]*models.TradeHistory, error)
}

type MongoTradeRepository struct {
	collection *mongo.Collection
}

func NewTradeRepository(client *mongo.Client, dbName, collectionName string) TradeRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoTradeRepository{collection: collection}
}

func (r *MongoTradeRepository) SaveTrade(trade *models.TradeHistory) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	trade.ID = primitive.NewObjectID()
	trade.OpenTime = time.Now()
	_, err := r.collection.InsertOne(ctx, trade)
	return err
}

func (r *MongoTradeRepository) GetTradeByID(id primitive.ObjectID) (*models.TradeHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var trade models.TradeHistory
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&trade)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &trade, err
}

func (r *MongoTradeRepository) GetTradesByUserID(userID primitive.ObjectID) ([]*models.TradeHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var trades []*models.TradeHistory
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &trades); err != nil {
		return nil, err
	}
	return trades, nil
}
