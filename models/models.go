package models

import (
	"time"

	"github.com/google/uuid"
)

// Account represents a bank account
type Account struct {
	ID        string    `json:"id" bson:"id"`
	OwnerName string    `json:"owner_name" bson:"ownername"`
	Balance   float64   `json:"balance" bson:"balance"`
	CreatedAt time.Time `json:"created_at" bson:"createdat"`
	UpdatedAt time.Time `json:"updated_at" bson:"updatedat"`
}

// Transaction represents a transaction log
type Transaction struct {
	ID              string    `json:"id" bson:"_id"`
	TransactionID   string    `json:"transaction_id" bson:"transactionid"`
	AccountID       string    `json:"account_id" bson:"accountid"`
	Type            string    `json:"type" bson:"type"` // "deposit" or "withdraw"
	Amount          float64   `json:"amount" bson:"amount"`
	PreviousBalance float64   `json:"previous_balance" bson:"previousbalance"`
	NewBalance      float64   `json:"new_balance" bson:"newbalance"`
	Description     string    `json:"description" bson:"description"`
	Timestamp       time.Time `json:"timestamp" bson:"timestamp"`
	Status          string    `json:"status" bson:"status"`                                  // "pending", "completed", "failed"
	ErrorMessage    string    `json:"error_message,omitempty" bson:"errormessage,omitempty"` // Added for failed transactions
}

// CreateAccountRequest represents the request body for creating an account
type CreateAccountRequest struct {
	OwnerName      string  `json:"owner_name"`
	InitialBalance float64 `json:"initial_balance"`
}

// TransactionRequest represents the request body for transactions
type TransactionRequest struct {
	Type        string  `json:"type"` // "deposit" or "withdraw"
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
}

// Helper functions to generate IDs
func NewAccountID() string {
	return "acc_" + uuid.New().String()
}

func NewTransactionID() string {
	return "txn_" + uuid.New().String()
}
