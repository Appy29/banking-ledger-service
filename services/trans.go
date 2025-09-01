package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/utils"
)

type TransactionService struct {
	accountStorage     AccountStorage
	transactionStorage TransactionStorage
}

func NewTransactionService(accountStorage AccountStorage, transactionStorage TransactionStorage) *TransactionService {
	return &TransactionService{
		accountStorage:     accountStorage,
		transactionStorage: transactionStorage,
	}
}

// GetAccountByID provides access to account information for validation
func (s *TransactionService) GetAccountByID(ctx context.Context, accountID string) (*models.Account, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "get_account_by_id"),
		slog.String("account_id", accountID))

	logger.Info("Getting account for transaction service")

	account, err := s.accountStorage.GetAccountByID(ctx, accountID)
	if err != nil {
		logger.Error("Failed to get account", slog.String("error", err.Error()))
		return nil, err
	}

	logger.Info("Account retrieved successfully", slog.Float64("balance", account.Balance))
	return account, nil
}

// CreatePendingTransaction saves a transaction with "pending" status
func (s *TransactionService) CreatePendingTransaction(ctx context.Context, transaction *models.Transaction) error {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "create_pending_transaction"),
		slog.String("transaction_id", transaction.TransactionID))

	logger.Info("Creating pending transaction",
		slog.String("type", transaction.Type),
		slog.Float64("amount", transaction.Amount))

	err := s.transactionStorage.CreateTransaction(ctx, transaction)
	if err != nil {
		logger.Error("Failed to create pending transaction", slog.String("error", err.Error()))
		return err
	}

	logger.Info("Pending transaction created successfully")
	return nil
}

// UpdateTransactionStatus updates the status of a transaction
func (s *TransactionService) UpdateTransactionStatus(ctx context.Context, transactionID, status string) error {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "update_transaction_status"),
		slog.String("transaction_id", transactionID))

	logger.Info("Updating transaction status", slog.String("new_status", status))

	err := s.transactionStorage.UpdateTransactionStatus(ctx, transactionID, status)
	if err != nil {
		logger.Error("Failed to update transaction status", slog.String("error", err.Error()))
		return err
	}

	logger.Info("Transaction status updated successfully")
	return nil
}

// UpdateTransactionStatusWithError updates status with error message
func (s *TransactionService) UpdateTransactionStatusWithError(ctx context.Context, transactionID, status, errorMessage string) error {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "update_transaction_status_with_error"),
		slog.String("transaction_id", transactionID))

	logger.Info("Updating transaction status with error",
		slog.String("new_status", status),
		slog.String("error_message", errorMessage))

	err := s.transactionStorage.UpdateTransactionStatusWithError(ctx, transactionID, status, errorMessage)
	if err != nil {
		logger.Error("Failed to update transaction status with error", slog.String("error", err.Error()))
		return err
	}

	logger.Info("Transaction status updated with error successfully")
	return nil
}

// ProcessTransaction - Updated to use atomic balance operations
func (s *TransactionService) ProcessTransaction(ctx context.Context, accountID string, req *models.TransactionRequest) (*models.Transaction, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "process_transaction_sync"),
		slog.String("account_id", accountID))

	logger.Info("Starting synchronous transaction processing",
		slog.String("type", req.Type),
		slog.Float64("amount", req.Amount))

	// Validate request
	if err := s.validateTransactionRequest(ctx, req); err != nil {
		return nil, err
	}

	// Use atomic balance update instead of read-calculate-write pattern
	previousBalance, newBalance, err := s.accountStorage.AtomicBalanceUpdate(ctx, accountID, req.Type, req.Amount)
	if err != nil {
		logger.Error("Atomic balance update failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to process transaction: %w", err)
	}

	logger.Info("Balance updated atomically",
		slog.Float64("previous_balance", previousBalance),
		slog.Float64("new_balance", newBalance))

	// Create transaction record
	transaction := &models.Transaction{
		ID:              models.NewTransactionID(),
		TransactionID:   models.NewTransactionID(),
		AccountID:       accountID,
		Type:            req.Type,
		Amount:          req.Amount,
		PreviousBalance: previousBalance,
		NewBalance:      newBalance,
		Description:     req.Description,
		Timestamp:       time.Now(),
		Status:          "completed",
	}

	logger = logger.With(slog.String("transaction_id", transaction.TransactionID))
	logger.Info("Creating transaction record")

	// Save transaction to MongoDB
	if err := s.transactionStorage.CreateTransaction(ctx, transaction); err != nil {
		logger.Error("Failed to save transaction, rolling back balance",
			slog.String("error", err.Error()),
			slog.Float64("rollback_balance", previousBalance))

		// Rollback balance update by reversing the transaction
		rollbackType := "withdraw"
		if req.Type == "withdraw" {
			rollbackType = "deposit"
		}
		s.accountStorage.AtomicBalanceUpdate(ctx, accountID, rollbackType, req.Amount)
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}

	logger.Info("Synchronous transaction completed successfully",
		slog.Float64("final_balance", newBalance))
	return transaction, nil
}

// ProcessTransactionAsync - Updated to use atomic operations for pending transactions
func (s *TransactionService) ProcessTransactionAsync(ctx context.Context, transactionID string, req *models.TransactionRequest) (*models.Transaction, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "process_transaction_async"),
		slog.String("transaction_id", transactionID))

	logger.Info("Starting async transaction processing",
		slog.String("type", req.Type),
		slog.Float64("amount", req.Amount))

	// Get the pending transaction
	transaction, err := s.GetTransactionByID(ctx, transactionID)
	if err != nil {
		logger.Error("Pending transaction not found", slog.String("error", err.Error()))
		return nil, fmt.Errorf("pending transaction not found: %w", err)
	}

	if transaction.Status != "pending" {
		logger.Error("Transaction not in pending state", slog.String("current_status", transaction.Status))
		return nil, fmt.Errorf("transaction is not in pending state: %s", transaction.Status)
	}

	logger.Info("Retrieved pending transaction", slog.String("account_id", transaction.AccountID))

	// Validate transaction request
	if err := s.validateTransactionRequest(ctx, req); err != nil {
		s.UpdateTransactionStatusWithError(ctx, transactionID, "failed", err.Error())
		return nil, err
	}

	// Use atomic balance update for async processing
	previousBalance, newBalance, err := s.accountStorage.AtomicBalanceUpdate(ctx, transaction.AccountID, req.Type, req.Amount)
	if err != nil {
		logger.Error("Async atomic balance update failed", slog.String("error", err.Error()))
		s.UpdateTransactionStatusWithError(ctx, transactionID, "failed", err.Error())
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	logger.Info("Balance calculated for async transaction",
		slog.Float64("previous_balance", previousBalance),
		slog.Float64("new_balance", newBalance))

	// Update transaction record with final values
	updatedTransaction := &models.Transaction{
		ID:              transaction.ID,
		TransactionID:   transaction.TransactionID,
		AccountID:       transaction.AccountID,
		Type:            transaction.Type,
		Amount:          transaction.Amount,
		PreviousBalance: previousBalance,
		NewBalance:      newBalance,
		Description:     transaction.Description,
		Timestamp:       transaction.Timestamp,
		Status:          "completed",
		ErrorMessage:    "",
	}

	logger.Info("Updating transaction to completed status")
	if err := s.transactionStorage.UpdateTransaction(ctx, updatedTransaction); err != nil {
		logger.Error("Failed to update transaction record, rolling back",
			slog.String("error", err.Error()),
			slog.Float64("rollback_balance", previousBalance))

		// Rollback balance update
		rollbackType := "withdraw"
		if req.Type == "withdraw" {
			rollbackType = "deposit"
		}
		s.accountStorage.AtomicBalanceUpdate(ctx, transaction.AccountID, rollbackType, req.Amount)
		s.UpdateTransactionStatusWithError(ctx, transactionID, "failed", "Failed to update transaction record")
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	logger.Info("Async transaction completed successfully",
		slog.Float64("final_balance", newBalance))
	return updatedTransaction, nil
}

func (s *TransactionService) GetTransactionHistory(ctx context.Context, accountID string, page, limit int) ([]models.Transaction, int64, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "get_transaction_history"),
		slog.String("account_id", accountID))

	logger.Info("Getting transaction history",
		slog.Int("page", page),
		slog.Int("limit", limit))

	if accountID == "" {
		logger.Error("Account ID is required for transaction history")
		return nil, 0, fmt.Errorf("account ID is required")
	}

	// Verify account exists
	_, err := s.accountStorage.GetAccountByID(ctx, accountID)
	if err != nil {
		logger.Error("Account not found for transaction history", slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("account not found: %w", err)
	}

	// Set default values if not provided
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	transactions, total, err := s.transactionStorage.GetTransactionsByAccountID(ctx, accountID, page, limit)
	if err != nil {
		logger.Error("Failed to get transaction history from storage", slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to get transaction history: %w", err)
	}

	logger.Info("Transaction history retrieved successfully",
		slog.Int64("total_count", total),
		slog.Int("returned_count", len(transactions)))

	return transactions, total, nil
}

func (s *TransactionService) GetTransactionByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "get_transaction_by_id"),
		slog.String("transaction_id", transactionID))

	logger.Info("Getting transaction by ID")

	if transactionID == "" {
		logger.Error("Transaction ID is required")
		return nil, fmt.Errorf("transaction ID is required")
	}

	transaction, err := s.transactionStorage.GetTransactionByID(ctx, transactionID)
	if err != nil {
		logger.Error("Failed to get transaction from storage", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	logger.Info("Transaction retrieved successfully",
		slog.String("status", transaction.Status),
		slog.String("account_id", transaction.AccountID))

	return transaction, nil
}

func (s *TransactionService) CreateInitialTransaction(ctx context.Context, accountID string, initialBalance float64) error {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "transaction"),
		slog.String("operation", "create_initial_transaction"),
		slog.String("account_id", accountID))

	if initialBalance <= 0 {
		logger.Info("No initial transaction needed", slog.Float64("balance", initialBalance))
		return nil
	}

	logger.Info("Creating initial transaction", slog.Float64("initial_balance", initialBalance))

	initialTransaction := &models.Transaction{
		ID:              models.NewTransactionID(),
		TransactionID:   models.NewTransactionID(),
		AccountID:       accountID,
		Type:            "deposit",
		Amount:          initialBalance,
		PreviousBalance: 0,
		NewBalance:      initialBalance,
		Description:     "Initial deposit",
		Timestamp:       time.Now(),
		Status:          "completed",
	}

	err := s.transactionStorage.CreateTransaction(ctx, initialTransaction)
	if err != nil {
		logger.Error("Failed to create initial transaction", slog.String("error", err.Error()))
		return err
	}

	logger.Info("Initial transaction created successfully")
	return nil
}

func (s *TransactionService) validateTransactionRequest(ctx context.Context, req *models.TransactionRequest) error {
	logger := utils.LoggerFromContext(ctx).With(slog.String("service", "transaction"))

	if req.Type != "deposit" && req.Type != "withdraw" {
		logger.Error("Invalid transaction type", slog.String("type", req.Type))
		return fmt.Errorf("transaction type must be either 'deposit' or 'withdraw'")
	}

	if req.Amount <= 0 {
		logger.Error("Invalid amount", slog.Float64("amount", req.Amount))
		return fmt.Errorf("amount must be greater than 0")
	}

	logger.Info("Transaction request validated successfully")
	return nil
}
