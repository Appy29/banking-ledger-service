//go:build integration
// +build integration

package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/appy29/banking-ledger-service/storage"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPostgresURI = "host=localhost port=5432 user=postgres password=postgres dbname=banking_ledger_test sslmode=disable"
	testMongoURI    = "mongodb://admin:admin@localhost:27017"
	testMongoDB     = "banking_test"
)

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func setupTestContext() context.Context {
	logger := setupTestLogger()
	return utils.WithLogger(context.Background(), logger)
}

// TestDatabaseIntegration tests PostgreSQL and MongoDB integration
func TestDatabaseIntegration(t *testing.T) {
	ctx := setupTestContext()

	// Setup PostgreSQL
	accountStorage, err := storage.NewPostgresAccountStorage(testPostgresURI)
	require.NoError(t, err, "Failed to connect to test PostgreSQL")
	defer accountStorage.Close()

	// Setup MongoDB
	transactionStorage, err := storage.NewMongoTransactionStorage(testMongoURI, testMongoDB, "test_transactions")
	require.NoError(t, err, "Failed to connect to test MongoDB")
	defer transactionStorage.Close()

	// Initialize services
	accountService := services.NewAccountService(accountStorage)
	transactionService := services.NewTransactionService(accountStorage, transactionStorage)

	t.Run("Account CRUD Operations", func(t *testing.T) {
		// Create account
		req := &models.CreateAccountRequest{
			OwnerName:      "Integration Test User",
			InitialBalance: 1000.00,
		}

		account, err := accountService.CreateAccount(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, account)
		assert.Equal(t, "Integration Test User", account.OwnerName)
		assert.Equal(t, 1000.00, account.Balance)

		// Retrieve account
		retrievedAccount, err := accountService.GetAccountByID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, account.ID, retrievedAccount.ID)
		assert.Equal(t, account.OwnerName, retrievedAccount.OwnerName)
		assert.Equal(t, account.Balance, retrievedAccount.Balance)

		// Get balance
		balance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 1000.00, balance)
	})

	t.Run("Transaction Processing with Database Persistence", func(t *testing.T) {
		// Create test account
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "Transaction Test User",
			InitialBalance: 500.00,
		}
		account, err := accountService.CreateAccount(ctx, accountReq)
		require.NoError(t, err)

		// Test deposit
		depositReq := &models.TransactionRequest{
			Type:        "deposit",
			Amount:      250.00,
			Description: "Integration test deposit",
		}

		transaction, err := transactionService.ProcessTransaction(ctx, account.ID, depositReq)
		require.NoError(t, err)
		assert.Equal(t, "deposit", transaction.Type)
		assert.Equal(t, 250.00, transaction.Amount)
		assert.Equal(t, 500.00, transaction.PreviousBalance)
		assert.Equal(t, 750.00, transaction.NewBalance)
		assert.Equal(t, "completed", transaction.Status)

		// Verify account balance was updated in PostgreSQL
		updatedBalance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 750.00, updatedBalance)

		// Verify transaction was saved in MongoDB
		savedTransaction, err := transactionService.GetTransactionByID(ctx, transaction.TransactionID)
		require.NoError(t, err)
		assert.Equal(t, transaction.TransactionID, savedTransaction.TransactionID)
		assert.Equal(t, "completed", savedTransaction.Status)

		// Test withdrawal
		withdrawReq := &models.TransactionRequest{
			Type:        "withdraw",
			Amount:      200.00,
			Description: "Integration test withdrawal",
		}

		withdrawTransaction, err := transactionService.ProcessTransaction(ctx, account.ID, withdrawReq)
		require.NoError(t, err)
		assert.Equal(t, 750.00, withdrawTransaction.PreviousBalance)
		assert.Equal(t, 550.00, withdrawTransaction.NewBalance)

		// Verify final balance
		finalBalance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 550.00, finalBalance)
	})

	t.Run("Transaction History with Pagination", func(t *testing.T) {
		// Create account for history test
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "History Test User",
			InitialBalance: 300.00,
		}
		account, err := accountService.CreateAccount(ctx, accountReq)
		require.NoError(t, err)

		// Create multiple transactions
		for i := 0; i < 5; i++ {
			req := &models.TransactionRequest{
				Type:        "deposit",
				Amount:      float64(10 * (i + 1)),
				Description: "Test transaction",
			}
			_, err := transactionService.ProcessTransaction(ctx, account.ID, req)
			require.NoError(t, err)
		}

		// Get transaction history
		transactions, total, err := transactionService.GetTransactionHistory(ctx, account.ID, 1, 3)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Equal(t, 3, len(transactions))

		// Verify transactions are in descending order (newest first)
		assert.True(t, transactions[0].Timestamp.After(transactions[1].Timestamp) ||
			transactions[0].Timestamp.Equal(transactions[1].Timestamp))
	})

	t.Run("Insufficient Funds Handling", func(t *testing.T) {
		// Create account with limited funds
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "Insufficient Funds Test",
			InitialBalance: 100.00,
		}
		account, err := accountService.CreateAccount(ctx, accountReq)
		require.NoError(t, err)

		// Try to withdraw more than available
		withdrawReq := &models.TransactionRequest{
			Type:        "withdraw",
			Amount:      150.00,
			Description: "Insufficient funds test",
		}

		transaction, err := transactionService.ProcessTransaction(ctx, account.ID, withdrawReq)
		assert.Error(t, err)
		assert.Nil(t, transaction)
		assert.Contains(t, err.Error(), "insufficient funds")

		// Verify balance unchanged
		balance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 100.00, balance)
	})

	t.Run("Concurrent Transaction Processing", func(t *testing.T) {
		// Create account for concurrency test
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "Concurrency Test User",
			InitialBalance: 1000.00,
		}
		account, err := accountService.CreateAccount(ctx, accountReq)
		require.NoError(t, err)

		// Run concurrent deposits
		numGoroutines := 10
		depositAmount := 50.00
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				req := &models.TransactionRequest{
					Type:        "deposit",
					Amount:      depositAmount,
					Description: "Concurrent deposit",
				}
				_, err := transactionService.ProcessTransaction(ctx, account.ID, req)
				if err != nil {
					errors <- err
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// Success
			case err := <-errors:
				t.Errorf("Concurrent transaction failed: %v", err)
			case <-time.After(30 * time.Second):
				t.Error("Timeout waiting for concurrent transactions")
			}
		}

		// Verify final balance
		finalBalance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		expectedBalance := 1000.00 + (float64(numGoroutines) * depositAmount)
		assert.Equal(t, expectedBalance, finalBalance)

		// Verify all transactions were recorded
		transactions, total, err := transactionService.GetTransactionHistory(ctx, account.ID, 1, 20)
		require.NoError(t, err)
		assert.Equal(t, int64(numGoroutines), total)
		assert.Equal(t, numGoroutines, len(transactions))
	})
}

// TestAsyncTransactionProcessing tests the pending transaction flow
func TestAsyncTransactionProcessing(t *testing.T) {
	ctx := setupTestContext()

	// Setup storage
	accountStorage, err := storage.NewPostgresAccountStorage(testPostgresURI)
	require.NoError(t, err)
	defer accountStorage.Close()

	transactionStorage, err := storage.NewMongoTransactionStorage(testMongoURI, testMongoDB, "test_async_transactions")
	require.NoError(t, err)
	defer transactionStorage.Close()

	// Initialize service
	transactionService := services.NewTransactionService(accountStorage, transactionStorage)
	accountService := services.NewAccountService(accountStorage)

	t.Run("Pending Transaction Creation and Processing", func(t *testing.T) {
		// Create test account
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "Async Test User",
			InitialBalance: 600.00,
		}
		account, err := accountService.CreateAccount(ctx, accountReq)
		require.NoError(t, err)

		// Create pending transaction
		transactionID := models.NewTransactionID()
		pendingTransaction := &models.Transaction{
			ID:              transactionID,
			TransactionID:   transactionID,
			AccountID:       account.ID,
			Type:            "deposit",
			Amount:          150.00,
			PreviousBalance: account.Balance,
			NewBalance:      0, // Will be updated during processing
			Description:     "Async deposit test",
			Timestamp:       time.Now(),
			Status:          "pending",
		}

		// Save pending transaction
		err = transactionService.CreatePendingTransaction(ctx, pendingTransaction)
		require.NoError(t, err)

		// Verify transaction is pending
		savedTransaction, err := transactionService.GetTransactionByID(ctx, transactionID)
		require.NoError(t, err)
		assert.Equal(t, "pending", savedTransaction.Status)
		assert.Equal(t, 0.0, savedTransaction.NewBalance) // Not yet processed

		// Process the pending transaction (simulating worker behavior)
		req := &models.TransactionRequest{
			Type:        "deposit",
			Amount:      150.00,
			Description: "Async deposit test",
		}

		completedTransaction, err := transactionService.ProcessTransactionAsync(ctx, transactionID, req)
		require.NoError(t, err)
		assert.Equal(t, "completed", completedTransaction.Status)
		assert.Equal(t, 600.00, completedTransaction.PreviousBalance)
		assert.Equal(t, 750.00, completedTransaction.NewBalance)

		// Verify account balance was updated
		finalBalance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 750.00, finalBalance)

		// Verify transaction record was updated
		finalTransaction, err := transactionService.GetTransactionByID(ctx, transactionID)
		require.NoError(t, err)
		assert.Equal(t, "completed", finalTransaction.Status)
		assert.Equal(t, 750.00, finalTransaction.NewBalance)
	})

	t.Run("Async Insufficient Funds Handling", func(t *testing.T) {
		// Create account with limited funds
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "Async Insufficient Test",
			InitialBalance: 100.00,
		}
		account, err := accountService.CreateAccount(ctx, accountReq)
		require.NoError(t, err)

		// Create pending withdrawal that will fail
		transactionID := models.NewTransactionID()
		pendingTransaction := &models.Transaction{
			ID:            transactionID,
			TransactionID: transactionID,
			AccountID:     account.ID,
			Type:          "withdraw",
			Amount:        200.00, // More than available
			Status:        "pending",
			Timestamp:     time.Now(),
		}

		err = transactionService.CreatePendingTransaction(ctx, pendingTransaction)
		require.NoError(t, err)

		// Try to process (should fail)
		req := &models.TransactionRequest{
			Type:   "withdraw",
			Amount: 200.00,
		}

		processedTransaction, err := transactionService.ProcessTransactionAsync(ctx, transactionID, req)
		assert.Error(t, err)
		assert.Nil(t, processedTransaction)
		assert.Contains(t, err.Error(), "insufficient funds")

		// Verify transaction status was updated to failed
		failedTransaction, err := transactionService.GetTransactionByID(ctx, transactionID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedTransaction.Status)

		// Verify account balance unchanged
		balance, err := accountService.GetAccountBalance(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 100.00, balance)
	})
}
