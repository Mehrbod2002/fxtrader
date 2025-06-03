package repository

import (
	"context"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CopyTradeRepository interface {
	SaveSubscription(subscription *models.CopyTradeSubscription) error
	GetSubscriptionByID(id primitive.ObjectID) (*models.CopyTradeSubscription, error)
	GetSubscriptionsByFollowerID(followerID string) ([]*models.CopyTradeSubscription, error)
	GetAllSubscriptions() ([]*models.CopyTradeSubscription, error)
	GetActiveSubscriptionsByLeaderID(leaderID string) ([]*models.CopyTradeSubscription, error)
	SaveCopyTrade(copyTrade *models.CopyTrade) error
}

type MongoCopyTradeRepository struct {
	collection *mongo.Collection
}

func NewCopyTradeRepository(client *mongo.Client, dbName, collectionName string) CopyTradeRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoCopyTradeRepository{collection: collection}
}

func (r *MongoCopyTradeRepository) SaveSubscription(subscription *models.CopyTradeSubscription) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subscription.ID = primitive.NewObjectID()
	subscription.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, subscription)
	return err
}

func (r *MongoCopyTradeRepository) GetSubscriptionByID(id primitive.ObjectID) (*models.CopyTradeSubscription, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var subscription models.CopyTradeSubscription
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&subscription)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &subscription, err
}

func (r *MongoCopyTradeRepository) GetAllSubscriptions() ([]*models.CopyTradeSubscription, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var subscriptions []*models.CopyTradeSubscription
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (r *MongoCopyTradeRepository) GetSubscriptionsByFollowerID(followerID string) ([]*models.CopyTradeSubscription, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var subscriptions []*models.CopyTradeSubscription
	cursor, err := r.collection.Find(ctx, bson.M{"follower_id": followerID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (r *MongoCopyTradeRepository) GetActiveSubscriptionsByLeaderID(leaderID string) ([]*models.CopyTradeSubscription, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var subscriptions []*models.CopyTradeSubscription
	cursor, err := r.collection.Find(ctx, bson.M{"leader_id": leaderID, "status": "ACTIVE"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (r *MongoCopyTradeRepository) SaveCopyTrade(copyTrade *models.CopyTrade) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	copyTrade.ID = primitive.NewObjectID()
	copyTrade.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, copyTrade)
	return err
}
