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

type TransactionRepository interface {
	SaveTransaction(transaction *models.Transaction) error
	GetTransactionByID(id primitive.ObjectID) (*models.Transaction, error)
	GetTransactionsByUserID(userID primitive.ObjectID) ([]*models.Transaction, error)
	GetAllTransactions() ([]*models.Transaction, error)
	UpdateTransaction(id primitive.ObjectID, transaction *models.Transaction) error
}

type MongoTransactionRepository struct {
	collection *mongo.Collection
}

func NewTransactionRepository(client *mongo.Client, dbName, collectionName string) TransactionRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoTransactionRepository{collection: collection}
}

func (r *MongoTransactionRepository) SaveTransaction(transaction *models.Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transaction.ID = primitive.NewObjectID()
	transaction.RequestTime = time.Now()
	_, err := r.collection.InsertOne(ctx, transaction)
	return err
}

func (r *MongoTransactionRepository) GetTransactionByID(id primitive.ObjectID) (*models.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var transaction models.Transaction
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&transaction)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &transaction, err
}

func (r *MongoTransactionRepository) GetTransactionsByUserID(userID primitive.ObjectID) ([]*models.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var transactions []*models.Transaction
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.M{"request_time": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *MongoTransactionRepository) GetAllTransactions() ([]*models.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var transactions []*models.Transaction
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"request_time": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *MongoTransactionRepository) UpdateTransaction(id primitive.ObjectID, transaction *models.Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":        transaction.Status,
			"response_time": transaction.ResponseTime,
			"admin_note":    transaction.AdminNote,
		},
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}
