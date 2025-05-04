package repository

import (
	"context"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type AdminRepository interface {
	SaveAdmin(admin *models.AdminAccount) error
	GetAdminByID(id primitive.ObjectID) (*models.AdminAccount, error)
	GetAdminByUsername(username string) (*models.AdminAccount, error)
}

type MongoAdminRepository struct {
	collection *mongo.Collection
}

func NewAdminRepository(client *mongo.Client, dbName, collectionName string) AdminRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoAdminRepository{collection: collection}
}

func (r *MongoAdminRepository) SaveAdmin(admin *models.AdminAccount) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.collection.InsertOne(ctx, admin)
	return err
}

func (r *MongoAdminRepository) GetAdminByID(id primitive.ObjectID) (*models.AdminAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var admin models.AdminAccount
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&admin)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *MongoAdminRepository) GetAdminByUsername(username string) (*models.AdminAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var admin models.AdminAccount
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&admin)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}
