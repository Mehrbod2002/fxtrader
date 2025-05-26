package repository

import (
	"context"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type LeaderRequestRepository interface {
	SaveLeaderRequest(request *models.LeaderRequest) error
	GetLeaderRequestByID(id primitive.ObjectID) (*models.LeaderRequest, error)
	GetPendingLeaderRequests() ([]*models.LeaderRequest, error)
	UpdateLeaderRequest(request *models.LeaderRequest) error
}

type MongoLeaderRequestRepository struct {
	collection *mongo.Collection
}

func NewLeaderRequestRepository(client *mongo.Client, dbName, collectionName string) LeaderRequestRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoLeaderRequestRepository{collection: collection}
}

func (r *MongoLeaderRequestRepository) SaveLeaderRequest(request *models.LeaderRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request.ID = primitive.NewObjectID()
	request.CreatedAt = time.Now()
	request.UpdatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, request)
	return err
}

func (r *MongoLeaderRequestRepository) GetLeaderRequestByID(id primitive.ObjectID) (*models.LeaderRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var request models.LeaderRequest
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&request)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &request, err
}

func (r *MongoLeaderRequestRepository) GetPendingLeaderRequests() ([]*models.LeaderRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var requests []*models.LeaderRequest
	cursor, err := r.collection.Find(ctx, bson.M{"status": "PENDING"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *MongoLeaderRequestRepository) UpdateLeaderRequest(request *models.LeaderRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": request.ID}, bson.M{"$set": request})
	return err
}
