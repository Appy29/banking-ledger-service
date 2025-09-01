package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoTransactionStorage struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
}

func NewMongoTransactionStorage(uri, database, collection string) (*MongoTransactionStorage, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	// Connect to MongoDB
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(database)
	coll := db.Collection(collection)

	// Create indexes for better performance
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "accountid", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "transactionid", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
	}

	_, err = coll.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	return &MongoTransactionStorage{
		client:     client,
		database:   db,
		collection: coll,
	}, nil
}

func (s *MongoTransactionStorage) CreateTransaction(ctx context.Context, transaction *models.Transaction) error {
	_, err := s.collection.InsertOne(ctx, transaction)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}
	return nil
}

// UpdateTransaction updates a transaction record
func (s *MongoTransactionStorage) UpdateTransaction(ctx context.Context, transaction *models.Transaction) error {
	filter := bson.M{"transactionid": transaction.TransactionID}

	update := bson.M{
		"$set": bson.M{
			"previousbalance": transaction.PreviousBalance,
			"newbalance":      transaction.NewBalance,
			"status":          transaction.Status,
			"errormessage":    transaction.ErrorMessage,
		},
	}

	result, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("transaction not found for update")
	}

	return nil
}

func (s *MongoTransactionStorage) GetTransactionsByAccountID(ctx context.Context, accountID string, page, limit int) ([]models.Transaction, int64, error) {
	// Calculate skip value for pagination
	skip := (page - 1) * limit

	// Create filter for account ID
	filter := bson.M{"accountid": accountID}

	// Get total count
	total, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Find transactions with pagination and sorting (newest first)
	findOptions := options.Find()
	findOptions.SetSkip(int64(skip))
	findOptions.SetLimit(int64(limit))
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}}) // Sort by timestamp descending

	cursor, err := s.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find transactions: %w", err)
	}
	defer cursor.Close(ctx)

	var transactions []models.Transaction
	if err := cursor.All(ctx, &transactions); err != nil {
		return nil, 0, fmt.Errorf("failed to decode transactions: %w", err)
	}

	return transactions, total, nil
}

func (s *MongoTransactionStorage) GetTransactionByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	// Search by transaction_id field - this is the key fix
	filter := bson.M{"transactionid": transactionID}

	var transaction models.Transaction
	err := s.collection.FindOne(ctx, filter).Decode(&transaction)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("transaction not found")
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	return &transaction, nil
}

func (s *MongoTransactionStorage) UpdateTransactionStatus(ctx context.Context, transactionID, status string) error {
	filter := bson.M{"transactionid": transactionID}
	update := bson.M{"$set": bson.M{"status": status}}

	result, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("transaction not found for status update")
	}

	return nil
}

// UpdateTransactionStatusWithError updates transaction status with error message
func (s *MongoTransactionStorage) UpdateTransactionStatusWithError(ctx context.Context, transactionID, status, errorMessage string) error {
	filter := bson.M{"transactionid": transactionID}
	update := bson.M{
		"$set": bson.M{
			"status":       status,
			"errormessage": errorMessage,
		},
	}

	result, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update transaction status with error: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("transaction not found for status update with error")
	}

	return nil
}

func (s *MongoTransactionStorage) Close() error {
	return s.client.Disconnect(context.Background())
}
