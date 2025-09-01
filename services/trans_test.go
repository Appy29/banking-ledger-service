package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestTransactionService_ProcessTransaction_DepositSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      250.00,
		Description: "Test deposit",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations - Updated to use AtomicBalanceUpdate
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, accountID, "deposit", 250.00).
		Return(500.00, 750.00, nil). // previousBalance, newBalance, error
		Times(1)

	mockTransactionStorage.EXPECT().
		CreateTransaction(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, transaction *models.Transaction) error {
			assert.Equal(t, accountID, transaction.AccountID)
			assert.Equal(t, "deposit", transaction.Type)
			assert.Equal(t, 250.00, transaction.Amount)
			assert.Equal(t, 500.00, transaction.PreviousBalance)
			assert.Equal(t, 750.00, transaction.NewBalance)
			assert.Equal(t, "completed", transaction.Status)
			return nil
		}).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransaction(ctx, accountID, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, transaction)
	assert.Equal(t, "deposit", transaction.Type)
	assert.Equal(t, 250.00, transaction.Amount)
	assert.Equal(t, 500.00, transaction.PreviousBalance)
	assert.Equal(t, 750.00, transaction.NewBalance)
	assert.Equal(t, "completed", transaction.Status)
}

func TestTransactionService_ProcessTransaction_WithdrawSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	req := &models.TransactionRequest{
		Type:        "withdraw",
		Amount:      200.00,
		Description: "Test withdrawal",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations - Updated to use AtomicBalanceUpdate
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, accountID, "withdraw", 200.00).
		Return(500.00, 300.00, nil). // previousBalance, newBalance, error
		Times(1)

	mockTransactionStorage.EXPECT().
		CreateTransaction(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, transaction *models.Transaction) error {
			assert.Equal(t, "withdraw", transaction.Type)
			assert.Equal(t, 200.00, transaction.Amount)
			assert.Equal(t, 500.00, transaction.PreviousBalance)
			assert.Equal(t, 300.00, transaction.NewBalance)
			return nil
		}).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransaction(ctx, accountID, req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "withdraw", transaction.Type)
	assert.Equal(t, 200.00, transaction.Amount)
	assert.Equal(t, 500.00, transaction.PreviousBalance)
	assert.Equal(t, 300.00, transaction.NewBalance)
}

func TestTransactionService_ProcessTransaction_InsufficientFunds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	req := &models.TransactionRequest{
		Type:        "withdraw",
		Amount:      600.00, // More than available balance
		Description: "Test insufficient funds",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations - AtomicBalanceUpdate returns insufficient funds error
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, accountID, "withdraw", 600.00).
		Return(0.0, 0.0, errors.New("insufficient funds: current balance 500.00, requested 600.00")).
		Times(1)

	// No transaction creation should occur since balance update failed

	// Execute
	transaction, err := service.ProcessTransaction(ctx, accountID, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "insufficient funds")
}

func TestTransactionService_ProcessTransaction_InvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	req := &models.TransactionRequest{
		Type:        "invalid",
		Amount:      100.00,
		Description: "Invalid transaction type",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Execute - validation should fail before any storage calls
	transaction, err := service.ProcessTransaction(ctx, "acc_123", req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "transaction type must be either 'deposit' or 'withdraw'")
}

func TestTransactionService_ProcessTransaction_NegativeAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      -100.00, // Negative amount
		Description: "Negative amount test",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Execute - validation should fail before any storage calls
	transaction, err := service.ProcessTransaction(ctx, "acc_123", req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "amount must be greater than 0")
}

func TestTransactionService_ProcessTransaction_TransactionStorageFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      100.00,
		Description: "Storage failure test",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations - Balance update succeeds but transaction save fails
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, accountID, "deposit", 100.00).
		Return(500.00, 600.00, nil).
		Times(1)

	mockTransactionStorage.EXPECT().
		CreateTransaction(ctx, gomock.Any()).
		Return(errors.New("database error")).
		Times(1)

	// Expect rollback - reverse the deposit with a withdraw
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, accountID, "withdraw", 100.00).
		Return(600.00, 500.00, nil).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransaction(ctx, accountID, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "failed to save transaction")
}

func TestTransactionService_ProcessTransactionAsync_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      150.00,
		Description: "Async deposit test",
	}

	pendingTransaction := &models.Transaction{
		ID:            transactionID,
		TransactionID: transactionID,
		AccountID:     "acc_12345",
		Type:          "deposit",
		Amount:        150.00,
		Status:        "pending",
		Timestamp:     time.Now(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations - Updated for async processing
	mockTransactionStorage.EXPECT().
		GetTransactionByID(ctx, transactionID).
		Return(pendingTransaction, nil).
		Times(1)

	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, "acc_12345", "deposit", 150.00).
		Return(400.00, 550.00, nil). // previousBalance, newBalance, error
		Times(1)

	mockTransactionStorage.EXPECT().
		UpdateTransaction(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, transaction *models.Transaction) error {
			assert.Equal(t, "completed", transaction.Status)
			assert.Equal(t, 400.00, transaction.PreviousBalance)
			assert.Equal(t, 550.00, transaction.NewBalance)
			return nil
		}).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransactionAsync(ctx, transactionID, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, transaction)
	assert.Equal(t, "completed", transaction.Status)
	assert.Equal(t, 550.00, transaction.NewBalance)
}

func TestTransactionService_ProcessTransactionAsync_NotPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      150.00,
		Description: "Already processed transaction",
	}

	completedTransaction := &models.Transaction{
		ID:            transactionID,
		TransactionID: transactionID,
		Status:        "completed", // Already completed
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockTransactionStorage.EXPECT().
		GetTransactionByID(ctx, transactionID).
		Return(completedTransaction, nil).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransactionAsync(ctx, transactionID, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "transaction is not in pending state: completed")
}

func TestTransactionService_ProcessTransactionAsync_InsufficientFunds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	req := &models.TransactionRequest{
		Type:        "withdraw",
		Amount:      600.00,
		Description: "Async insufficient funds test",
	}

	pendingTransaction := &models.Transaction{
		ID:            transactionID,
		TransactionID: transactionID,
		AccountID:     "acc_12345",
		Type:          "withdraw",
		Amount:        600.00,
		Status:        "pending",
		Timestamp:     time.Now(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations
	mockTransactionStorage.EXPECT().
		GetTransactionByID(ctx, transactionID).
		Return(pendingTransaction, nil).
		Times(1)

	// AtomicBalanceUpdate fails with insufficient funds
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, "acc_12345", "withdraw", 600.00).
		Return(0.0, 0.0, errors.New("insufficient funds: current balance 200.00, requested 600.00")).
		Times(1)

	// Expect transaction status update with error
	mockTransactionStorage.EXPECT().
		UpdateTransactionStatusWithError(ctx, transactionID, "failed", gomock.Any()).
		Return(nil).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransactionAsync(ctx, transactionID, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "insufficient funds")
}

func TestTransactionService_ProcessTransactionAsync_StorageRollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      100.00,
		Description: "Rollback test",
	}

	pendingTransaction := &models.Transaction{
		ID:            transactionID,
		TransactionID: transactionID,
		AccountID:     "acc_12345",
		Type:          "deposit",
		Amount:        100.00,
		Status:        "pending",
		Timestamp:     time.Now(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations
	mockTransactionStorage.EXPECT().
		GetTransactionByID(ctx, transactionID).
		Return(pendingTransaction, nil).
		Times(1)

	// Balance update succeeds
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, "acc_12345", "deposit", 100.00).
		Return(500.00, 600.00, nil).
		Times(1)

	// Transaction update fails
	mockTransactionStorage.EXPECT().
		UpdateTransaction(ctx, gomock.Any()).
		Return(errors.New("database connection lost")).
		Times(1)

	// Expect rollback - reverse the deposit
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, "acc_12345", "withdraw", 100.00).
		Return(600.00, 500.00, nil).
		Times(1)

	// Expect error status update
	mockTransactionStorage.EXPECT().
		UpdateTransactionStatusWithError(ctx, transactionID, "failed", "Failed to update transaction record").
		Return(nil).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransactionAsync(ctx, transactionID, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "failed to update transaction")
}

func TestTransactionService_GetTransactionByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	expectedTransaction := &models.Transaction{
		ID:            transactionID,
		TransactionID: transactionID,
		AccountID:     "acc_12345",
		Type:          "deposit",
		Amount:        200.00,
		Status:        "completed",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockTransactionStorage.EXPECT().
		GetTransactionByID(ctx, transactionID).
		Return(expectedTransaction, nil).
		Times(1)

	// Execute
	transaction, err := service.GetTransactionByID(ctx, transactionID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedTransaction, transaction)
}

func TestTransactionService_GetTransactionByID_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Execute with empty ID
	transaction, err := service.GetTransactionByID(ctx, "")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "transaction ID is required")
}

func TestTransactionService_GetTransactionHistory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	page := 1
	limit := 10

	existingAccount := &models.Account{
		ID:      accountID,
		Balance: 500.00,
	}

	expectedTransactions := []models.Transaction{
		{ID: "txn_1", AccountID: accountID, Type: "deposit", Amount: 100.00},
		{ID: "txn_2", AccountID: accountID, Type: "withdraw", Amount: 50.00},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockAccountStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(existingAccount, nil).
		Times(1)

	mockTransactionStorage.EXPECT().
		GetTransactionsByAccountID(ctx, accountID, page, limit).
		Return(expectedTransactions, int64(2), nil).
		Times(1)

	// Execute
	transactions, total, err := service.GetTransactionHistory(ctx, accountID, page, limit)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedTransactions, transactions)
	assert.Equal(t, int64(2), total)
}

func TestTransactionService_GetTransactionHistory_AccountNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_nonexistent"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockAccountStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(nil, errors.New("account not found")).
		Times(1)

	// Execute
	transactions, total, err := service.GetTransactionHistory(ctx, accountID, 1, 10)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transactions)
	assert.Equal(t, int64(0), total)
	assert.Contains(t, err.Error(), "account not found")
}

func TestTransactionService_GetTransactionHistory_PaginationDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	existingAccount := &models.Account{ID: accountID, Balance: 500.00}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockAccountStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(existingAccount, nil).
		Times(1)

	// Expect default pagination values (page=1, limit=10)
	mockTransactionStorage.EXPECT().
		GetTransactionsByAccountID(ctx, accountID, 1, 10).
		Return([]models.Transaction{}, int64(0), nil).
		Times(1)

	// Execute with invalid pagination (should use defaults)
	transactions, total, err := service.GetTransactionHistory(ctx, accountID, 0, -5)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, []models.Transaction{}, transactions)
	assert.Equal(t, int64(0), total)
}

func TestTransactionService_CreatePendingTransaction_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	pendingTransaction := &models.Transaction{
		ID:            "txn_12345",
		TransactionID: "txn_12345",
		AccountID:     "acc_12345",
		Type:          "deposit",
		Amount:        300.00,
		Status:        "pending",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockTransactionStorage.EXPECT().
		CreateTransaction(ctx, pendingTransaction).
		Return(nil).
		Times(1)

	// Execute
	err := service.CreatePendingTransaction(ctx, pendingTransaction)

	// Assert
	assert.NoError(t, err)
}

func TestTransactionService_UpdateTransactionStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	status := "completed"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockTransactionStorage.EXPECT().
		UpdateTransactionStatus(ctx, transactionID, status).
		Return(nil).
		Times(1)

	// Execute
	err := service.UpdateTransactionStatus(ctx, transactionID, status)

	// Assert
	assert.NoError(t, err)
}

func TestTransactionService_UpdateTransactionStatusWithError_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	transactionID := "txn_12345"
	status := "failed"
	errorMessage := "Insufficient funds"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockTransactionStorage.EXPECT().
		UpdateTransactionStatusWithError(ctx, transactionID, status, errorMessage).
		Return(nil).
		Times(1)

	// Execute
	err := service.UpdateTransactionStatusWithError(ctx, transactionID, status, errorMessage)

	// Assert
	assert.NoError(t, err)
}

func TestTransactionService_CreateInitialTransaction_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	initialBalance := 1000.00

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockTransactionStorage.EXPECT().
		CreateTransaction(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, transaction *models.Transaction) error {
			assert.Equal(t, accountID, transaction.AccountID)
			assert.Equal(t, "deposit", transaction.Type)
			assert.Equal(t, 1000.00, transaction.Amount)
			assert.Equal(t, 0.0, transaction.PreviousBalance)
			assert.Equal(t, 1000.00, transaction.NewBalance)
			assert.Equal(t, "Initial deposit", transaction.Description)
			assert.Equal(t, "completed", transaction.Status)
			return nil
		}).
		Times(1)

	// Execute
	err := service.CreateInitialTransaction(ctx, accountID, initialBalance)

	// Assert
	assert.NoError(t, err)
}

func TestTransactionService_CreateInitialTransaction_ZeroBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	initialBalance := 0.0

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// No storage call expected for zero balance
	// Execute
	err := service.CreateInitialTransaction(ctx, accountID, initialBalance)

	// Assert
	assert.NoError(t, err)
}

func TestTransactionService_GetAccountByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	expectedAccount := &models.Account{
		ID:        accountID,
		OwnerName: "Test User",
		Balance:   750.00,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockAccountStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(expectedAccount, nil).
		Times(1)

	// Execute
	account, err := service.GetAccountByID(ctx, accountID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedAccount, account)
}

// Additional edge case tests
func TestTransactionService_ProcessTransaction_ZeroAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      0.0, // Zero amount should fail validation
		Description: "Zero amount test",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Execute - validation should fail before any storage calls
	transaction, err := service.ProcessTransaction(ctx, "acc_123", req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "amount must be greater than 0")
}

func TestTransactionService_ProcessTransaction_EmptyAccountID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      100.00,
		Description: "Empty account ID test",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectation for empty account ID
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, "", "deposit", 100.00).
		Return(0.0, 0.0, errors.New("account not found")).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransaction(ctx, "", req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, transaction)
	assert.Contains(t, err.Error(), "failed to process transaction")
}

func TestTransactionService_ProcessTransaction_LargeAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	accountID := "acc_12345"
	largeAmount := 999999999.99

	req := &models.TransactionRequest{
		Type:        "deposit",
		Amount:      largeAmount,
		Description: "Large amount test",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectations for large amount
	mockAccountStorage.EXPECT().
		AtomicBalanceUpdate(ctx, accountID, "deposit", largeAmount).
		Return(1000.00, 1000000000.99, nil).
		Times(1)

	mockTransactionStorage.EXPECT().
		CreateTransaction(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, transaction *models.Transaction) error {
			assert.Equal(t, largeAmount, transaction.Amount)
			assert.Equal(t, 1000.00, transaction.PreviousBalance)
			assert.Equal(t, 1000000000.99, transaction.NewBalance)
			return nil
		}).
		Times(1)

	// Execute
	transaction, err := service.ProcessTransaction(ctx, accountID, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, transaction)
	assert.Equal(t, largeAmount, transaction.Amount)
}

func TestTransactionService_ValidateTransactionRequest_AllScenarios(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountStorage := NewMockAccountStorage(ctrl)
	mockTransactionStorage := NewMockTransactionStorage(ctrl)
	service := NewTransactionService(mockAccountStorage, mockTransactionStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	testCases := []struct {
		name          string
		request       *models.TransactionRequest
		expectedError string
	}{
		{
			name: "Valid deposit",
			request: &models.TransactionRequest{
				Type:   "deposit",
				Amount: 100.00,
			},
			expectedError: "",
		},
		{
			name: "Valid withdraw",
			request: &models.TransactionRequest{
				Type:   "withdraw",
				Amount: 50.00,
			},
			expectedError: "",
		},
		{
			name: "Invalid type",
			request: &models.TransactionRequest{
				Type:   "transfer",
				Amount: 100.00,
			},
			expectedError: "transaction type must be either 'deposit' or 'withdraw'",
		},
		{
			name: "Negative amount",
			request: &models.TransactionRequest{
				Type:   "deposit",
				Amount: -50.00,
			},
			expectedError: "amount must be greater than 0",
		},
		{
			name: "Zero amount",
			request: &models.TransactionRequest{
				Type:   "deposit",
				Amount: 0.0,
			},
			expectedError: "amount must be greater than 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// If we expect an error, no storage calls should be made
			if tc.expectedError != "" {
				// Execute
				transaction, err := service.ProcessTransaction(ctx, "acc_123", tc.request)

				// Assert
				assert.Error(t, err)
				assert.Nil(t, transaction)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				// For valid requests, we need to mock storage calls
				mockAccountStorage.EXPECT().
					AtomicBalanceUpdate(ctx, "acc_123", tc.request.Type, tc.request.Amount).
					Return(500.00, 500.00+tc.request.Amount, nil).
					Times(1)

				mockTransactionStorage.EXPECT().
					CreateTransaction(ctx, gomock.Any()).
					Return(nil).
					Times(1)

				// Execute
				transaction, err := service.ProcessTransaction(ctx, "acc_123", tc.request)

				// Assert
				assert.NoError(t, err)
				assert.NotNil(t, transaction)
				assert.Equal(t, tc.request.Type, transaction.Type)
				assert.Equal(t, tc.request.Amount, transaction.Amount)
			}
		})
	}
}
