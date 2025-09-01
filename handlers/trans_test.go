package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/queue"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTransactionService for testing
type MockTransactionService struct {
	mock.Mock
}

func (m *MockTransactionService) ProcessTransaction(ctx context.Context, accountID string, req *models.TransactionRequest) (*models.Transaction, error) {
	args := m.Called(ctx, accountID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

func (m *MockTransactionService) GetTransactionHistory(ctx context.Context, accountID string, page, limit int) ([]models.Transaction, int64, error) {
	args := m.Called(ctx, accountID, page, limit)
	return args.Get(0).([]models.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockTransactionService) GetTransactionByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

func (m *MockTransactionService) GetAccountByID(ctx context.Context, accountID string) (*models.Account, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Account), args.Error(1)
}

func (m *MockTransactionService) CreatePendingTransaction(ctx context.Context, transaction *models.Transaction) error {
	args := m.Called(ctx, transaction)
	return args.Error(0)
}

func (m *MockTransactionService) ProcessTransactionAsync(ctx context.Context, transactionID string, req *models.TransactionRequest) (*models.Transaction, error) {
	args := m.Called(ctx, transactionID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

func (m *MockTransactionService) UpdateTransactionStatus(ctx context.Context, transactionID, status string) error {
	args := m.Called(ctx, transactionID, status)
	return args.Error(0)
}

func (m *MockTransactionService) UpdateTransactionStatusWithError(ctx context.Context, transactionID, status, errorMessage string) error {
	args := m.Called(ctx, transactionID, status, errorMessage)
	return args.Error(0)
}

func (m *MockTransactionService) CreateInitialTransaction(ctx context.Context, accountID string, initialBalance float64) error {
	args := m.Called(ctx, accountID, initialBalance)
	return args.Error(0)
}

func setupTransactionTestRouter(asyncMode bool) (*gin.Engine, *MockTransactionService) {
	gin.SetMode(gin.TestMode)

	mockService := &MockTransactionService{}
	// Use nil for RabbitMQ in sync mode tests, create a simple mock for async tests
	var rabbitMQ *queue.RabbitMQ = nil

	handler := NewTransactionHandler(mockService, rabbitMQ, asyncMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		ctx := utils.WithLogger(c.Request.Context(), logger)
		c.Request = c.Request.WithContext(ctx)

		// Set pagination defaults
		if page := c.Query("page"); page != "" {
			if p, err := strconv.Atoi(page); err == nil {
				c.Set("page", p)
			}
		} else {
			c.Set("page", 1)
		}
		if limit := c.Query("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil {
				c.Set("limit", l)
			}
		} else {
			c.Set("limit", 10)
		}
		c.Next()
	})

	router.POST("/accounts/:id/transactions", handler.ProcessTransaction)
	router.GET("/accounts/:id/transactions", handler.GetTransactions)
	router.GET("/transactions/:id", handler.GetTransaction)
	router.GET("/processing-mode", handler.GetProcessingMode)

	return router, mockService
}

func TestProcessTransaction_SyncMode_Success(t *testing.T) {
	router, mockService := setupTransactionTestRouter(false) // Sync mode

	expectedTransaction := &models.Transaction{
		ID:              "txn_12345",
		TransactionID:   "txn_12345",
		AccountID:       "acc_12345",
		Type:            "deposit",
		Amount:          500.00,
		PreviousBalance: 1000.00,
		NewBalance:      1500.00,
		Status:          "completed",
		Timestamp:       time.Now(),
	}

	mockService.On("ProcessTransaction", mock.Anything, "acc_12345", mock.MatchedBy(func(req *models.TransactionRequest) bool {
		return req.Type == "deposit" && req.Amount == 500.00
	})).Return(expectedTransaction, nil)

	requestBody := models.TransactionRequest{
		Type:        "deposit",
		Amount:      500.00,
		Description: "Test deposit",
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/accounts/acc_12345/transactions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Transaction processed successfully", response["message"])
	assert.Equal(t, "sync", response["processing_mode"])

	mockService.AssertExpectations(t)
}

func TestProcessTransaction_InvalidJSON(t *testing.T) {
	router, _ := setupTransactionTestRouter(false)

	req, _ := http.NewRequest("POST", "/accounts/acc_12345/transactions", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request body", response["error"])
}

func TestGetTransactions_Success(t *testing.T) {
	router, mockService := setupTransactionTestRouter(false)

	expectedTransactions := []models.Transaction{
		{
			ID:        "txn_1",
			AccountID: "acc_12345",
			Type:      "deposit",
			Amount:    500.00,
			Status:    "completed",
		},
	}

	mockService.On("GetTransactionHistory", mock.Anything, "acc_12345", 1, 10).Return(expectedTransactions, int64(1), nil)

	req, _ := http.NewRequest("GET", "/accounts/acc_12345/transactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "transactions")
	assert.Contains(t, response, "pagination")

	mockService.AssertExpectations(t)
}

func TestGetTransaction_Success(t *testing.T) {
	router, mockService := setupTransactionTestRouter(false)

	expectedTransaction := &models.Transaction{
		ID:        "txn_12345",
		AccountID: "acc_12345",
		Type:      "deposit",
		Amount:    500.00,
		Status:    "completed",
	}

	mockService.On("GetTransactionByID", mock.Anything, "txn_12345").Return(expectedTransaction, nil)

	req, _ := http.NewRequest("GET", "/transactions/txn_12345", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "transaction")

	mockService.AssertExpectations(t)
}

func TestGetTransaction_NotFound(t *testing.T) {
	router, mockService := setupTransactionTestRouter(false)

	mockService.On("GetTransactionByID", mock.Anything, "txn_nonexistent").Return(nil, errors.New("transaction not found"))

	req, _ := http.NewRequest("GET", "/transactions/txn_nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

func TestGetProcessingMode_SyncMode(t *testing.T) {
	router, _ := setupTransactionTestRouter(false) // Sync mode

	req, _ := http.NewRequest("GET", "/processing-mode", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "sync", response["processing_mode"])
	assert.Equal(t, "disconnected", response["queue_status"])
	assert.Equal(t, false, response["async_enabled"])
}
