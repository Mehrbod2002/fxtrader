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

type AlertRepository interface {
	SaveAlert(alert *models.Alert) error
	GetAlertByID(id primitive.ObjectID) (*models.Alert, error)
	GetAlertsByUserID(userID string) ([]*models.Alert, error)
	GetPendingAlerts() ([]*models.Alert, error)
	UpdateAlert(id primitive.ObjectID, alert *models.Alert) error
}

type MongoAlertRepository struct {
	collection *mongo.Collection
}

func NewAlertRepository(client *mongo.Client, dbName, collectionName string) AlertRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoAlertRepository{collection: collection}
}

func (r *MongoAlertRepository) SaveAlert(alert *models.Alert) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	alert.ID = primitive.NewObjectID()
	alert.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, alert)
	return err
}

func (r *MongoAlertRepository) GetAlertByID(id primitive.ObjectID) (*models.Alert, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var alert models.Alert
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&alert)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &alert, err
}

func (r *MongoAlertRepository) GetAlertsByUserID(userID string) ([]*models.Alert, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var alerts []*models.Alert
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

func (r *MongoAlertRepository) GetPendingAlerts() ([]*models.Alert, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var alerts []*models.Alert
	cursor, err := r.collection.Find(ctx, bson.M{"status": models.AlertStatusPending}, options.Find())
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

func (r *MongoAlertRepository) UpdateAlert(id primitive.ObjectID, alert *models.Alert) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":       alert.Status,
			"triggered_at": alert.TriggeredAt,
		},
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}
