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
	SaveUser(user *models.UserAccount) error
	GetUserByID(id primitive.ObjectID) (*models.UserAccount, error)
	GetUserByTelegramID(telegramID string) (*models.UserAccount, error)
	GetAllUsers() ([]*models.UserAccount, error)
	GetUsersByLeaderStatus(isLeader bool) ([]*models.UserAccount, error)
	UpdateUser(user *models.UserAccount) error
	EditUser(user *models.UserAccount) error
	GetUserByReferralCode(code string) (*models.UserAccount, error)
	GetUsersReferredBy(code string, page, limit int64) ([]*models.UserAccount, int64, error)
	GetAllReferrals(page, limit int64) ([]*models.UserAccount, int64, error)
	GetUserAccounts(userID string) ([]*models.UserAccount, error)
	DeleteAccount(userID string, accountID primitive.ObjectID) error
	TransferBalance(sourceID, destID primitive.ObjectID, amount float64, sourceType, destType string) error
}

type MongoUserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(client *mongo.Client, dbName, collectionName string) UserRepository {
	collection := client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.M{"referral_code": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.M{"referred_by": 1},
		},
	})

	if err != nil {
		fmt.Printf("Failed to create indexes: %v\n", err)
	}

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

	switch user.AccountType {
	case "demo":
		user.DemoMT5Balance = 10000.0
		user.RealMT5Balance = 0.0
		user.Balance = 0.0
	case "real":
		user.DemoMT5Balance = 0.0
		user.RealMT5Balance = 0.0
		user.Balance = 0.0
	case "main":
		user.DemoMT5Balance = 0.0
		user.RealMT5Balance = 0.0
		user.Balance = 0.0
	}

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
	err := r.collection.FindOne(ctx, bson.M{"telegram_id": telegramID, "account_type": "main"}).Decode(&user)
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

func (r *MongoUserRepository) GetUserByReferralCode(code string) (*models.UserAccount, error) {
	filter := bson.M{"referral_code": code}
	var user models.UserAccount
	err := r.collection.FindOne(context.TODO(), filter).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (r *MongoUserRepository) GetUsersReferredBy(code string, page, limit int64) ([]*models.UserAccount, int64, error) {
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

	var users []*models.UserAccount
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *MongoUserRepository) GetAllReferrals(page, limit int64) ([]*models.UserAccount, int64, error) {
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

	var users []*models.UserAccount
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *MongoUserRepository) GetUserAccounts(userID string) ([]*models.UserAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"telegram_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var accounts []*models.UserAccount
	if err := cursor.All(ctx, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *MongoUserRepository) DeleteAccount(userID string, accountID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": accountID, "telegram_id": userID})
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("account not found")
	}
	return nil
}

func (r *MongoUserRepository) TransferBalance(sourceID, destID primitive.ObjectID, amount float64, sourceType, destType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := r.collection.Database().Client().StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {
		var source, dest models.UserAccount
		if err := r.collection.FindOne(sessionContext, bson.M{"_id": sourceID}).Decode(&source); err != nil {
			return nil, fmt.Errorf("source account not found: %w", err)
		}
		if err := r.collection.FindOne(sessionContext, bson.M{"_id": destID}).Decode(&dest); err != nil {
			return nil, fmt.Errorf("destination account not found: %w", err)
		}

		validSourceType := false
		for _, t := range source.AccountTypes {
			if t == sourceType {
				validSourceType = true
				break
			}
		}
		if !validSourceType {
			return nil, fmt.Errorf("source account does not have type: %s", sourceType)
		}

		validDestType := false
		for _, t := range dest.AccountTypes {
			if t == destType {
				validDestType = true
				break
			}
		}
		if !validDestType {
			return nil, fmt.Errorf("destination account does not have type: %s", destType)
		}

		if source.IsCopyTradeLeader && sourceType != "main" {
			return nil, fmt.Errorf("transfers from copy trade leader accounts are restricted")
		}
		if dest.IsCopyTradeLeader && destType != "main" {
			return nil, fmt.Errorf("transfers to copy trade leader accounts are restricted")
		}

		var sourceBalance *float64
		switch sourceType {
		case "main":
			sourceBalance = &source.Balance
		case "demo":
			sourceBalance = &source.DemoMT5Balance
		case "real":
			sourceBalance = &source.RealMT5Balance
		default:
			return nil, fmt.Errorf("invalid source account type: %s", sourceType)
		}

		var destBalance *float64
		switch destType {
		case "main":
			destBalance = &dest.Balance
		case "demo":
			destBalance = &dest.DemoMT5Balance
		case "real":
			destBalance = &dest.RealMT5Balance
		default:
			return nil, fmt.Errorf("invalid destination account type: %s", destType)
		}

		if *sourceBalance < amount {
			return nil, fmt.Errorf("insufficient balance in source account")
		}

		*sourceBalance -= amount
		*destBalance += amount

		if _, err := r.collection.UpdateOne(sessionContext, bson.M{"_id": sourceID}, bson.M{"$set": source}); err != nil {
			return nil, fmt.Errorf("failed to update source account: %w", err)
		}
		if _, err := r.collection.UpdateOne(sessionContext, bson.M{"_id": destID}, bson.M{"$set": dest}); err != nil {
			return nil, fmt.Errorf("failed to update destination account: %w", err)
		}

		return nil, nil
	}

	_, err = session.WithTransaction(ctx, callback)
	return err
}
