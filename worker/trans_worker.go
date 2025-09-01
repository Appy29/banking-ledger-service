package worker

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/queue"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/streadway/amqp"
)

// TransactionWorker processes transaction messages from the queue
type TransactionWorker struct {
	id             int
	rabbitmq       *queue.RabbitMQ
	transactionSvc *services.TransactionService
	accountSvc     *services.AccountService
	processingChan <-chan amqp.Delivery
}

// NewTransactionWorker creates a new transaction worker
func NewTransactionWorker(
	id int,
	rabbitmq *queue.RabbitMQ,
	transactionSvc *services.TransactionService,
	accountSvc *services.AccountService,
) *TransactionWorker {
	return &TransactionWorker{
		id:             id,
		rabbitmq:       rabbitmq,
		transactionSvc: transactionSvc,
		accountSvc:     accountSvc,
	}
}

// Start starts the worker to process messages
func (w *TransactionWorker) Start(ctx context.Context) error {
	var err error
	w.processingChan, err = w.rabbitmq.ConsumeTransactions()
	if err != nil {
		return err
	}

	log.Printf("Worker %d started and waiting for messages", w.id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d shutting down", w.id)
			return ctx.Err()
		case delivery := <-w.processingChan:
			w.processMessage(delivery)
		}
	}
}

// processMessage processes a single transaction message
func (w *TransactionWorker) processMessage(delivery amqp.Delivery) {
	start := time.Now()
	var msg queue.TransactionMessage

	// Parse the message
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		log.Printf("Worker %d: Failed to unmarshal message: %v", w.id, err)
		delivery.Nack(false, false) // Don't requeue malformed messages
		return
	}

	log.Printf("Worker %d: Processing transaction %s for account %s", w.id, msg.ID, msg.AccountID)

	// Create transaction request
	req := &models.TransactionRequest{
		Type:        msg.Type,
		Amount:      msg.Amount,
		Description: msg.Reference,
	}

	// Process the pending transaction using new method
	processedTransaction, processErr := w.transactionSvc.ProcessTransactionAsync(
		context.Background(),
		msg.ID, // This is the transaction ID we created earlier
		req,
	)

	if processErr != nil {
		log.Printf("Worker %d: Failed to process transaction %s: %v", w.id, msg.ID, processErr)

		// Transaction status is already updated to "failed" by the service
		// For business logic errors (insufficient funds, etc.), don't requeue
		if isBusinessError(processErr) {
			delivery.Ack(false)
			w.handleFailedTransaction(msg, processErr)
		} else {
			// For system errors (DB connection, etc.), requeue
			delivery.Nack(false, true)
		}
		return
	}

	// Successfully processed
	log.Printf("Worker %d: Successfully processed transaction %s in %v",
		w.id, processedTransaction.ID, time.Since(start))

	delivery.Ack(false)
}

// isBusinessError checks if error is due to business logic (don't requeue)
func isBusinessError(err error) bool {
	errorMsg := strings.ToLower(err.Error())
	businessErrors := []string{
		"insufficient funds",
		"invalid transaction type",
		"account not found",
		"amount must be greater than 0",
		"transaction is not in pending state",
	}

	for _, busErr := range businessErrors {
		if strings.Contains(errorMsg, busErr) {
			return true
		}
	}
	return false
}

// handleFailedTransaction logs failed transactions (could save to DB for audit)
func (w *TransactionWorker) handleFailedTransaction(msg queue.TransactionMessage, err error) {
	log.Printf("Worker %d: Transaction %s failed permanently: %v", w.id, msg.ID, err)

	// TODO: Additional logging or metrics could be added here
	// For example, increment a failed transaction counter
}
