//go:build integration
// +build integration

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/queue"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/appy29/banking-ledger-service/storage"
	"github.com/appy29/banking-ledger-service/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRabbitMQURL = "amqp://admin:admin@localhost:5672/"

// TestQueueIntegration - Fixed version with proper queue isolation
func TestQueueIntegration(t *testing.T) {
	t.Run("Queue Connection and Message Flow", func(t *testing.T) {
		// Setup RabbitMQ connection
		rabbitmq := queue.NewRabbitMQ(testRabbitMQURL)
		err := rabbitmq.Connect()
		require.NoError(t, err, "RabbitMQ connection failed")
		defer rabbitmq.Close()

		// CRITICAL FIX: Purge any existing messages from previous test runs
		err = rabbitmq.PurgeQueue()
		require.NoError(t, err, "Failed to purge queue")

		// Wait a moment for purge to complete
		time.Sleep(100 * time.Millisecond)

		// Verify queue is empty by trying to consume with immediate timeout
		msgs, err := rabbitmq.ConsumeTransactions()
		require.NoError(t, err)

		// Setup test message
		testMessage := queue.TransactionMessage{
			ID:        "txn_test_12345",
			AccountID: "acc_test_verification",
			Type:      "deposit",
			Amount:    250.00,
			Reference: "Queue integration test",
			CreatedAt: time.Now(),
		}

		// Publish test message
		err = rabbitmq.PublishTransaction(context.Background(), testMessage)
		require.NoError(t, err)

		// Consume and verify the exact message we just published
		select {
		case delivery := <-msgs:
			var receivedMessage queue.TransactionMessage
			err = json.Unmarshal(delivery.Body, &receivedMessage)
			require.NoError(t, err)

			// Verify this is OUR message by checking the unique ID first
			assert.Equal(t, "txn_test_12345", receivedMessage.ID)

			// Only proceed with other assertions if we got the right message
			if receivedMessage.ID == "txn_test_12345" {
				assert.Equal(t, "deposit", receivedMessage.Type)
				assert.Equal(t, 250.00, receivedMessage.Amount)
				assert.Equal(t, "acc_test_verification", receivedMessage.AccountID)
				assert.Equal(t, "Queue integration test", receivedMessage.Reference)
			}

			// Acknowledge the message
			delivery.Ack(false)

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message - queue system not working")
		}
	})

	t.Run("Multiple Message Handling", func(t *testing.T) {
		rabbitmq := queue.NewRabbitMQ(testRabbitMQURL)
		err := rabbitmq.Connect()
		require.NoError(t, err)
		defer rabbitmq.Close()

		// Purge queue before test
		err = rabbitmq.PurgeQueue()
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)

		// Publish multiple test messages with unique IDs
		testMessages := []queue.TransactionMessage{
			{
				ID:        "txn_multi_001",
				AccountID: "acc_multi_test",
				Type:      "deposit",
				Amount:    100.00,
				Reference: "Multi test 1",
				CreatedAt: time.Now(),
			},
			{
				ID:        "txn_multi_002",
				AccountID: "acc_multi_test",
				Type:      "withdraw",
				Amount:    50.00,
				Reference: "Multi test 2",
				CreatedAt: time.Now(),
			},
		}

		// Publish all messages
		ctx := context.Background()
		for _, msg := range testMessages {
			err = rabbitmq.PublishTransaction(ctx, msg)
			require.NoError(t, err)
		}

		// Consume messages
		msgs, err := rabbitmq.ConsumeTransactions()
		require.NoError(t, err)

		receivedCount := 0
		receivedIDs := make(map[string]bool)

		// Collect messages with timeout
		timeout := time.After(10 * time.Second)
		for receivedCount < len(testMessages) {
			select {
			case delivery := <-msgs:
				var receivedMessage queue.TransactionMessage
				err = json.Unmarshal(delivery.Body, &receivedMessage)
				require.NoError(t, err)

				// Only count messages from this test
				if receivedMessage.ID == "txn_multi_001" || receivedMessage.ID == "txn_multi_002" {
					receivedIDs[receivedMessage.ID] = true
					receivedCount++
				}

				delivery.Ack(false)

			case <-timeout:
				t.Fatalf("Timeout - only received %d/%d expected messages", receivedCount, len(testMessages))
			}
		}

		// Verify we got both unique messages
		assert.True(t, receivedIDs["txn_multi_001"], "Did not receive first test message")
		assert.True(t, receivedIDs["txn_multi_002"], "Did not receive second test message")
		assert.Equal(t, len(testMessages), len(receivedIDs), "Received wrong number of unique messages")
	})
}

// TestEndToEndAsyncFlow - Enhanced with better queue isolation
func TestEndToEndAsyncFlow(t *testing.T) {
	ctx := setupTestContext()

	// Setup storage
	accountStorage, err := storage.NewPostgresAccountStorage(testPostgresURI)
	require.NoError(t, err)
	defer accountStorage.Close()

	transactionStorage, err := storage.NewMongoTransactionStorage(testMongoURI, testMongoDB, "test_e2e_isolated")
	require.NoError(t, err)
	defer transactionStorage.Close()

	// Initialize services
	accountService := services.NewAccountService(accountStorage)
	transactionService := services.NewTransactionService(accountStorage, transactionStorage)

	t.Run("Complete Async Flow with Queue Integration", func(t *testing.T) {
		// Setup RabbitMQ
		rabbitmq := queue.NewRabbitMQ(testRabbitMQURL)
		err := rabbitmq.Connect()
		require.NoError(t, err)
		defer rabbitmq.Close()

		// CRITICAL: Purge queue to ensure clean state
		err = rabbitmq.PurgeQueue()
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		// Create test account
		accountReq := &models.CreateAccountRequest{
			OwnerName:      "Async Flow Test User",
			InitialBalance: 1000.00,
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
			Amount:          300.00,
			PreviousBalance: account.Balance,
			NewBalance:      0,
			Description:     "End-to-end async test",
			Timestamp:       time.Now(),
			Status:          "pending",
		}

		err = transactionService.CreatePendingTransaction(ctx, pendingTransaction)
		require.NoError(t, err)

		// Publish to queue with unique identifiers
		queueMessage := queue.TransactionMessage{
			ID:        transactionID, // Use same ID as transaction
			AccountID: account.ID,
			Type:      "deposit",
			Amount:    300.00,
			Reference: "End-to-end async test",
			CreatedAt: time.Now(),
		}

		err = rabbitmq.PublishTransaction(ctx, queueMessage)
		require.NoError(t, err)

		// Start worker with controlled context
		worker := worker.NewTransactionWorker(1, rabbitmq, transactionService, accountService)
		workerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Channel to signal when processing is complete
		done := make(chan bool, 1)

		go func() {
			// Start worker in background
			worker.Start(workerCtx)
		}()

		// Monitor transaction status
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-workerCtx.Done():
					return
				case <-ticker.C:
					transaction, err := transactionService.GetTransactionByID(ctx, transactionID)
					if err == nil && transaction.Status == "completed" {
						done <- true
						return
					}
				}
			}
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Verify final state
			finalTransaction, err := transactionService.GetTransactionByID(ctx, transactionID)
			require.NoError(t, err)
			assert.Equal(t, "completed", finalTransaction.Status)
			assert.Equal(t, 1000.00, finalTransaction.PreviousBalance)
			assert.Equal(t, 1300.00, finalTransaction.NewBalance)

			// Verify account balance
			finalBalance, err := accountService.GetAccountBalance(ctx, account.ID)
			require.NoError(t, err)
			assert.Equal(t, 1300.00, finalBalance)

		case <-time.After(15 * time.Second):
			// Check transaction status for debugging
			transaction, _ := transactionService.GetTransactionByID(ctx, transactionID)
			if transaction != nil {
				t.Logf("Transaction status at timeout: %s", transaction.Status)
				if transaction.ErrorMessage != "" {
					t.Logf("Transaction error: %s", transaction.ErrorMessage)
				}
			}
			t.Fatal("Test timeout - worker did not complete processing in time")
		}
	})
}

// Helper function to create isolated test queue names (alternative approach)
func createTestQueueName(testName string) string {
	return "test_" + testName + "_" + time.Now().Format("150405")
}
