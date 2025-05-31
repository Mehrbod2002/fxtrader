package repository

import (
	"context"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TradeRepository interface {
	SaveTrade(trade *models.TradeHistory) error
	GetTradeByID(id primitive.ObjectID) (*models.TradeHistory, error)
	GetTradesByUserID(userID primitive.ObjectID) ([]*models.TradeHistory, error)
	GetAllTrades() ([]*models.TradeHistory, error)
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

	if trade.ID.IsZero() {
		trade.ID = primitive.NewObjectID()
		trade.OpenTime = time.Now()
		_, err := r.collection.InsertOne(ctx, trade)
		return err
	}

	filter := bson.M{"_id": trade.ID}
	update := bson.M{
		"$set": bson.M{
			"status":           trade.Status,
			"matched_trade_id": trade.MatchedTradeID,
			"close_time":       trade.CloseTime,
			"stop_loss":        trade.StopLoss,
			"user_id":          trade.UserID,
			"take_profit":      trade.TakeProfit,
			"expiration":       trade.Expiration,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
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

	now := time.Now()
	for _, trade := range trades {
		if trade.Expiration != nil && trade.Expiration.Before(now) && trade.Status == "PENDING" {
			trade.Status = "EXPIRED"
			if _, err := r.collection.UpdateOne(ctx, bson.M{"_id": trade.ID}, bson.M{"$set": bson.M{"status": "EXPIRED"}}); err != nil {
				return nil, err
			}
		}
	}
	return trades, nil
}

func (r *MongoTradeRepository) GetAllTrades() ([]*models.TradeHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var trades []*models.TradeHistory
	if err := cursor.All(ctx, &trades); err != nil {
		return nil, err
	}
	return trades, nil
}
