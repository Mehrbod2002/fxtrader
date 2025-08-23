package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserRepository interface {
	Collection() *mongo.Collection
	SaveUser(user *models.User) error
	GetUserByID(id primitive.ObjectID) (*models.User, error)
	GetUserByUsername(username string) (*models.User, error)
	GetUserByTelegramID(telegramID string) (*models.User, error)
	GetAllUsers() ([]*models.User, error)
	GetUsersByLeaderStatus(isLeader bool) ([]*models.User, error)
	UpdateUser(user *models.User) error
	EditUser(user *models.User) error
	GetUserByReferralCode(code string) (*models.User, error)
	GetUsersReferredBy(code string, page, limit int64) ([]*models.User, int64, error)
	GetAllReferrals(page, limit int64) ([]*models.User, int64, error)
}

type MongoUserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(client *mongo.Client, dbName, collectionName string) UserRepository {
	collection := client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"username": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"telegram_id": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"referral_code": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"referred_by": 1}},
	})
	if err != nil {
		fmt.Printf("Failed to create indexes: %v\n", err)
	}

	return &MongoUserRepository{collection: collection}
}

func (r *MongoUserRepository) Collection() *mongo.Collection {
	return r.collection
}

func (r *MongoUserRepository) EditUser(user *models.User) error {
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
			"residence":                    user.Residence,
			"birthday":                     user.BirthDay,
			"telegram_id":                  user.TelegramID,
			"referral_code":                user.ReferralCode,
			"referred_by":                  user.ReferredBy,
			"registration_date":            user.RegistrationDate,
			"is_active":                    user.IsActive,
			"is_copy_trade_leader":         user.IsCopyTradeLeader,
			"is_copy_pending_trade_leader": user.IsCopyPendingTradeLeader,
			"balance":                      user.Balance,
			"bonus":                        user.Bonus,
			"leverage":                     user.Leverage,
			"trade_type":                   user.TradeType,
			"wallet_address":               user.WalletAddress,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("no user found with ID: %s", user.ID.Hex())
	}
	return nil
}

func (r *MongoUserRepository) UpdateUser(user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": user}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
	return err
}

func (r *MongoUserRepository) SaveUser(user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user.Balance = 0.0
	user.Bonus = 0.0

	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *MongoUserRepository) GetUserByID(id primitive.ObjectID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (r *MongoUserRepository) GetUserByUsername(username string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user by username: %w", err)
	}
	return &user, err
}

func (r *MongoUserRepository) GetUserByTelegramID(telegramID string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (r *MongoUserRepository) GetAllUsers() ([]*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *MongoUserRepository) GetUsersByLeaderStatus(isLeader bool) ([]*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var users []*models.User
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

func (r *MongoUserRepository) GetUserByReferralCode(code string) (*models.User, error) {
	filter := bson.M{"referral_code": code}
	var user models.User
	err := r.collection.FindOne(context.TODO(), filter).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (r *MongoUserRepository) GetUsersReferredBy(code string, page, limit int64) ([]*models.User, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Count total referred users
	total, err := r.collection.CountDocuments(ctx, bson.M{"referred_by": code})
	if err != nil {
		return nil, 0, err
	}

	// Fetch paginated referred users
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(skip).SetLimit(limit)
	cursor, err := r.collection.Find(ctx, bson.M{"referred_by": code}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var users []*models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *MongoUserRepository) GetAllReferrals(page, limit int64) ([]*models.User, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	skip := (page - 1) * limit
	opts := options.Find().SetSkip(skip).SetLimit(limit)
	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var users []*models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

type AccountRepository interface {
	Collection() *mongo.Collection
	SaveAccount(account *models.Account) error
	GetAccountByID(id primitive.ObjectID) (*models.Account, error)
	GetAccountByName(name string) (*models.Account, error)
	GetAccountsByUserID(userID primitive.ObjectID) ([]*models.Account, error)
	DeleteAccount(accountID, userID primitive.ObjectID) error
	UpdateAccount(account *models.Account) error
}

type MongoAccountRepository struct {
	collection *mongo.Collection
}

func NewAccountRepository(client *mongo.Client, dbName, collectionName string) AccountRepository {
	collection := client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"user_id": 1}},
		{Keys: bson.M{"account_name": 1, "user_id": 1}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		fmt.Printf("Failed to create indexes: %v\n", err)
	}

	return &MongoAccountRepository{collection: collection}
}

func (r *MongoAccountRepository) Collection() *mongo.Collection {
	return r.collection
}

func (r *MongoAccountRepository) SaveAccount(account *models.Account) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if account.AccountType != "demo" && account.AccountType != "real" {
		return fmt.Errorf("invalid account type: %s", account.AccountType)
	}

	account.Balance = 0.0

	_, err := r.collection.InsertOne(ctx, account)
	return err
}

func (r *MongoAccountRepository) GetAccountByID(id primitive.ObjectID) (*models.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var account models.Account
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&account)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &account, err
}

func (r *MongoAccountRepository) GetAccountByName(name string) (*models.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var account models.Account
	err := r.collection.FindOne(ctx, bson.M{"account_name": name}).Decode(&account)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account by name: %w", err)
	}
	return &account, err
}

func (r *MongoAccountRepository) GetAccountsByUserID(userID primitive.ObjectID) ([]*models.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var accounts []*models.Account
	if err := cursor.All(ctx, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *MongoAccountRepository) DeleteAccount(accountID, userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": accountID, "user_id": userID})
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("account not found")
	}
	return nil
}

func (r *MongoAccountRepository) UpdateAccount(account *models.Account) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": account}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": account.ID}, update)
	return err
}
