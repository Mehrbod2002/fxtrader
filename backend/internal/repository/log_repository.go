package repository

import (
	"context"
	"fxtrader/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogRepository interface {
	SaveLog(log *models.LogEntry) error
	GetAllLogs() ([]*models.LogEntry, error)
	GetLogsByUserID(userID primitive.ObjectID) ([]*models.LogEntry, error)
}

type MongoLogRepository struct {
	collection *mongo.Collection
}

func NewLogRepository(client *mongo.Client, dbName, collectionName string) LogRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoLogRepository{collection: collection}
}

func (r *MongoLogRepository) SaveLog(log *models.LogEntry) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.ID = primitive.NewObjectID()
	log.Timestamp = time.Now()
	_, err := r.collection.InsertOne(ctx, log)
	return err
}

func (r *MongoLogRepository) GetAllLogs() ([]*models.LogEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var logs []*models.LogEntry
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"timestamp": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *MongoLogRepository) GetLogsByUserID(userID primitive.ObjectID) ([]*models.LogEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var logs []*models.LogEntry
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.M{"timestamp": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}
