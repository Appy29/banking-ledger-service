package services

import (
	"context"

	"github.com/appy29/banking-ledger-service/models"
)

// AccountStorage defines the interface for account storage operations
type AccountStorage interface {
	CreateAccount(ctx context.Context, account *models.Account) error
	GetAccountByID(ctx context.Context, accountID string) (*models.Account, error)
	UpdateBalance(ctx context.Context, accountID string, newBalance float64) error
	AtomicBalanceUpdate(ctx context.Context, accountID, transactionType string, amount float64) (previousBalance, newBalance float64, err error)
}

// TransactionStorage defines the interface for transaction storage operations
type TransactionStorage interface {
	CreateTransaction(ctx context.Context, transaction *models.Transaction) error
	UpdateTransaction(ctx context.Context, transaction *models.Transaction) error
	GetTransactionsByAccountID(ctx context.Context, accountID string, page, limit int) ([]models.Transaction, int64, error)
	GetTransactionByID(ctx context.Context, transactionID string) (*models.Transaction, error)
	UpdateTransactionStatus(ctx context.Context, transactionID, status string) error
	UpdateTransactionStatusWithError(ctx context.Context, transactionID, status, errorMessage string) error
}

// AccountServiceInterface defines the contract for account operations
type AccountServiceInterface interface {
	CreateAccount(ctx context.Context, req *models.CreateAccountRequest) (*models.Account, error)
	GetAccountByID(ctx context.Context, accountID string) (*models.Account, error)
	GetAccountBalance(ctx context.Context, accountID string) (float64, error)
}

// TransactionServiceInterface defines the contract for transaction operations
type TransactionServiceInterface interface {
	// Synchronous operations
	ProcessTransaction(ctx context.Context, accountID string, req *models.TransactionRequest) (*models.Transaction, error)
	GetTransactionHistory(ctx context.Context, accountID string, page, limit int) ([]models.Transaction, int64, error)
	GetTransactionByID(ctx context.Context, transactionID string) (*models.Transaction, error)

	// Asynchronous operations
	CreatePendingTransaction(ctx context.Context, transaction *models.Transaction) error
	ProcessTransactionAsync(ctx context.Context, transactionID string, req *models.TransactionRequest) (*models.Transaction, error)
	UpdateTransactionStatus(ctx context.Context, transactionID, status string) error
	UpdateTransactionStatusWithError(ctx context.Context, transactionID, status, errorMessage string) error

	// Utility methods
	GetAccountByID(ctx context.Context, accountID string) (*models.Account, error)
	CreateInitialTransaction(ctx context.Context, accountID string, initialBalance float64) error
}
