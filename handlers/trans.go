package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/queue"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/gin-gonic/gin"
)

type TransactionHandler struct {
	transactionService services.TransactionServiceInterface
	rabbitMQ           *queue.RabbitMQ
	asyncMode          bool
}

func NewTransactionHandler(transactionService services.TransactionServiceInterface, rabbitMQ *queue.RabbitMQ, asyncMode bool) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
		rabbitMQ:           rabbitMQ,
		asyncMode:          asyncMode,
	}
}

// validateTransactionType validates the transaction type
func validateTransactionType(transactionType string) error {
	// Clean the input
	transactionType = strings.ToLower(strings.TrimSpace(transactionType))

	if transactionType == "" {
		return errors.New("transaction type is required")
	}

	if transactionType != "deposit" && transactionType != "withdraw" {
		return errors.New("transaction type must be either 'deposit' or 'withdraw'")
	}

	return nil
}

// validateTransactionAmount validates the transaction amount
func validateTransactionAmount(amount float64) error {
	if amount <= 0 {
		return errors.New("transaction amount must be greater than 0")
	}

	// Check for reasonable maximum transaction limit
	if amount > 999999999.99 {
		return errors.New("transaction amount exceeds maximum allowed limit")
	}

	// Check for precision
	// Use a small epsilon to handle floating point precision issues
	rounded := float64(int64(amount*100+0.5)) / 100
	if abs(amount-rounded) > 0.001 {
		return errors.New("transaction amount cannot have more than 2 decimal places")
	}

	// Check for minimum transaction amount (e.g., $0.01)
	if amount < 0.01 {
		return errors.New("transaction amount must be at least $0.01")
	}

	return nil
}

// validateTransactionRequest validates the complete transaction request
func validateTransactionRequest(req *models.TransactionRequest) error {
	// Validate transaction type
	if err := validateTransactionType(req.Type); err != nil {
		return err
	}

	// Validate amount
	if err := validateTransactionAmount(req.Amount); err != nil {
		return err
	}

	// Clean up description
	req.Description = strings.TrimSpace(req.Description)

	req.Type = strings.ToLower(strings.TrimSpace(req.Type))

	return nil
}

// ProcessTransaction handles POST /accounts/:id/transactions
func (h *TransactionHandler) ProcessTransaction(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)
	accountID := c.Param("id")

	logger = logger.With(
		slog.String("operation", "process_transaction"),
		slog.String("account_id", accountID),
	)

	var req models.TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid request body", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate transaction request
	if err := validateTransactionRequest(&req); err != nil {
		logger.Error("Transaction validation failed",
			slog.String("type", req.Type),
			slog.Float64("amount", req.Amount),
			slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid transaction request",
			"details": err.Error(),
		})
		return
	}

	logger = logger.With(
		slog.String("transaction_type", req.Type),
		slog.Float64("amount", req.Amount),
	)

	logger.Info("Transaction request received and validated")

	// If async mode is enabled and RabbitMQ is available, use queue
	if h.asyncMode && h.rabbitMQ != nil && h.rabbitMQ.IsConnected() {
		logger.Info("Processing transaction asynchronously")
		h.processTransactionAsync(c, accountID, &req)
	} else {
		logger.Info("Processing transaction synchronously")
		h.processTransactionSync(c, accountID, &req)
	}
}

// processTransactionAsync queues the transaction and creates pending record
func (h *TransactionHandler) processTransactionAsync(c *gin.Context, accountID string, req *models.TransactionRequest) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	// Validate account exists first
	account, err := h.transactionService.GetAccountByID(ctx, accountID)
	if err != nil {
		logger.Error("Account validation failed", slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Account not found",
		})
		return
	}

	logger.Info("Account validated for async transaction",
		slog.Float64("current_balance", account.Balance))

	// Pre-validate withdrawal amount against current balance
	if req.Type == "withdraw" && account.Balance < req.Amount {
		logger.Error("Insufficient funds detected before queueing",
			slog.Float64("current_balance", account.Balance),
			slog.Float64("requested_amount", req.Amount))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Insufficient funds",
			"details": "Current balance is insufficient for this withdrawal",
		})
		return
	}

	// Create transaction ID
	transactionID := models.NewTransactionID()
	logger = logger.With(slog.String("transaction_id", transactionID))

	// Save pending transaction to MongoDB immediately
	pendingTransaction := &models.Transaction{
		ID:              transactionID,
		TransactionID:   transactionID,
		AccountID:       accountID,
		Type:            req.Type,
		Amount:          req.Amount,
		PreviousBalance: account.Balance,
		NewBalance:      0, // Will be updated by worker
		Description:     req.Description,
		Timestamp:       time.Now(),
		Status:          "pending",
	}

	logger.Info("Creating pending transaction record")
	if err := h.transactionService.CreatePendingTransaction(ctx, pendingTransaction); err != nil {
		logger.Error("Failed to create pending transaction", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create transaction record",
		})
		return
	}

	// Create queue message
	message := queue.TransactionMessage{
		ID:        transactionID,
		AccountID: accountID,
		Type:      req.Type,
		Amount:    req.Amount,
		Reference: req.Description,
		CreatedAt: time.Now(),
	}

	logger.Info("Publishing transaction to queue")
	if err := h.rabbitMQ.PublishTransaction(ctx, message); err != nil {
		logger.Error("Failed to publish to queue, updating to failed status", slog.String("error", err.Error()))
		h.transactionService.UpdateTransactionStatusWithError(ctx, transactionID, "failed", "Queue system unavailable")
		h.processTransactionSync(c, accountID, req)
		return
	}

	logger.Info("Transaction queued successfully")
	c.JSON(http.StatusAccepted, gin.H{
		"message":         "Transaction queued for processing",
		"transaction_id":  transactionID,
		"status":          "pending",
		"account_id":      accountID,
		"processing_mode": "async",
	})
}

// processTransactionSync processes transaction synchronously
func (h *TransactionHandler) processTransactionSync(c *gin.Context, accountID string, req *models.TransactionRequest) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	logger.Info("Processing transaction synchronously")

	transaction, err := h.transactionService.ProcessTransaction(ctx, accountID, req)
	if err != nil {
		logger.Error("Synchronous transaction processing failed", slog.String("error", err.Error()))

		// Provide more specific error messages for common issues
		if strings.Contains(err.Error(), "insufficient funds") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Insufficient funds",
				"details": err.Error(),
			})
		} else if strings.Contains(err.Error(), "account not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Account not found",
				"details": err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Transaction processing failed",
				"details": err.Error(),
			})
		}
		return
	}

	logger.Info("Transaction processed successfully",
		slog.String("transaction_id", transaction.ID),
		slog.Float64("new_balance", transaction.NewBalance))

	c.JSON(http.StatusOK, gin.H{
		"message":         "Transaction processed successfully",
		"transaction":     transaction,
		"processing_mode": "sync",
	})
}

// GetTransactions handles GET /accounts/:id/transactions
func (h *TransactionHandler) GetTransactions(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)
	accountID := c.Param("id")

	// Get pagination parameters from middleware
	page := c.GetInt("page")
	limit := c.GetInt("limit")
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 10
	}

	logger = logger.With(
		slog.String("operation", "get_transactions"),
		slog.String("account_id", accountID),
		slog.Int("page", page),
		slog.Int("limit", limit),
	)

	logger.Info("Getting transaction history")

	transactions, total, err := h.transactionService.GetTransactionHistory(ctx, accountID, page, limit)
	if err != nil {
		logger.Error("Failed to get transaction history", slog.String("error", err.Error()))

		if strings.Contains(err.Error(), "account not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Account not found",
				"details": err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to retrieve transaction history",
				"details": err.Error(),
			})
		}
		return
	}

	logger.Info("Transaction history retrieved successfully",
		slog.Int64("total_transactions", total),
		slog.Int("returned_count", len(transactions)))

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GetTransaction handles GET /transactions/:id
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)
	transactionID := c.Param("id")

	logger = logger.With(
		slog.String("operation", "get_transaction"),
		slog.String("transaction_id", transactionID),
	)

	logger.Info("Getting individual transaction")

	transaction, err := h.transactionService.GetTransactionByID(ctx, transactionID)
	if err != nil {
		logger.Error("Failed to get transaction", slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("Transaction retrieved successfully",
		slog.String("status", transaction.Status),
		slog.String("account_id", transaction.AccountID))

	c.JSON(http.StatusOK, gin.H{
		"transaction": transaction,
	})
}

// GetProcessingMode handles GET /processing-mode
func (h *TransactionHandler) GetProcessingMode(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	logger.Info("Getting processing mode information")

	mode := "sync"
	queueStatus := "disconnected"

	if h.asyncMode {
		mode = "async"
		if h.rabbitMQ != nil && h.rabbitMQ.IsConnected() {
			queueStatus = "connected"
		}
	}

	logger.Info("Processing mode retrieved",
		slog.String("mode", mode),
		slog.String("queue_status", queueStatus))

	c.JSON(http.StatusOK, gin.H{
		"processing_mode": mode,
		"queue_status":    queueStatus,
		"async_enabled":   h.asyncMode,
	})
}
