package repository

// package repository
// (No changes needed; provided for reference)
import (
	"context"
	"fmt"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepository interface {
	SaveUser(user *models.UserAccount) error
	GetUserByID(id primitive.ObjectID) (*models.UserAccount, error)
	GetUserByTelegramID(telegramID string) (*models.UserAccount, error)
	GetAllUsers() ([]*models.UserAccount, error)
	GetUsersByLeaderStatus(isLeader bool) ([]*models.UserAccount, error)
	UpdateUser(user *models.UserAccount) error
	EditUser(user *models.UserAccount) error
}

type MongoUserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(client *mongo.Client, dbName, collectionName string) UserRepository {
	collection := client.Database(dbName).Collection(collectionName)
	return &MongoUserRepository{collection: collection}
}

func (r *MongoUserRepository) EditUser(user *models.UserAccount) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"username":                     user.Username,
			"full_name":                    user.FullName,
			"phone_number":                 user.PhoneNumber,
			"card_number":                  user.CardNumber,
			"national_id":                  user.NationalID,
			"citizenship":                  user.Citizenship,
			"account_type":                 user.AccountType,
			"account_types":                user.AccountTypes,
			"account_name":                 user.AccountName,
			"residence":                    user.Residence,
			"balance":                      user.Balance,
			"demo_mt5_balance":             user.DemoMT5Balance,
			"real_mt5_balance":             user.RealMT5Balance,
			"bonus":                        user.Bonus,
			"leverage":                     user.Leverage,
			"trade_type":                   user.TradeType,
			"wallet_address":               user.WalletAddress,
			"telegram_id":                  user.TelegramID,
			"birthday":                     user.BirthDay,
			"is_active":                    user.IsActive,
			"is_copy_trade_leader":         user.IsCopyTradeLeader,
			"is_copy_pending_trade_leader": user.IsCopyPendingTradeLeader,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("no user found with ID: %s", user.ID.Hex())
	}
	if result.ModifiedCount == 0 {
		return fmt.Errorf("no changes applied to user with ID: %s", user.ID.Hex())
	}

	return nil
}

func (r *MongoUserRepository) UpdateUser(user *models.UserAccount) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": user}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
	return err
}

func (r *MongoUserRepository) SaveUser(user *models.UserAccount) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user.DemoMT5Balance = 500
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *MongoUserRepository) GetUserByID(id primitive.ObjectID) (*models.UserAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.UserAccount
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *MongoUserRepository) GetUserByTelegramID(telegramID string) (*models.UserAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.UserAccount
	err := r.collection.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *MongoUserRepository) GetAllUsers() ([]*models.UserAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*models.UserAccount
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *MongoUserRepository) GetUsersByLeaderStatus(isLeader bool) ([]*models.UserAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var users []*models.UserAccount
	cursor, err := r.collection.Find(ctx, bson.M{"is_copy_trade_leader": isLeader})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}
