package repository

import (
	"context"
	"fxtrader/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RuleRepository interface {
	SaveRule(rule *models.Rule) error
	GetRuleByID(id primitive.ObjectID) (*models.Rule, error)
	GetAllRules() ([]*models.Rule, error)
	UpdateRule(id primitive.ObjectID, rule *models.Rule) error
	DeleteRule(id primitive.ObjectID) error
}

type MongoRuleRepository struct {
	collection *mongo.Collection
}

func NewRuleRepository(client *mongo.Client, dbName, collectionName string) RuleRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoRuleRepository{collection: collection}
}

func (r *MongoRuleRepository) SaveRule(rule *models.Rule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rule.ID = primitive.NewObjectID()
	rule.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, rule)
	return err
}

func (r *MongoRuleRepository) GetRuleByID(id primitive.ObjectID) (*models.Rule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rule models.Rule
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&rule)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &rule, err
}

func (r *MongoRuleRepository) GetAllRules() ([]*models.Rule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rules []*models.Rule
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *MongoRuleRepository) UpdateRule(id primitive.ObjectID, rule *models.Rule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{"content": rule.Content}}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *MongoRuleRepository) DeleteRule(id primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
