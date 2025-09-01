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
	"testing"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAccountService for testing
type MockAccountService struct {
	mock.Mock
}

func (m *MockAccountService) CreateAccount(ctx context.Context, req *models.CreateAccountRequest) (*models.Account, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Account), args.Error(1)
}

func (m *MockAccountService) GetAccountByID(ctx context.Context, accountID string) (*models.Account, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Account), args.Error(1)
}

func (m *MockAccountService) GetAccountBalance(ctx context.Context, accountID string) (float64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(float64), args.Error(1)
}

func setupAccountTestRouter() (*gin.Engine, *MockAccountService) {
	gin.SetMode(gin.TestMode)

	mockService := &MockAccountService{}
	handler := NewAccountHandler(mockService)

	router := gin.New()
	// Add logger middleware for testing
	router.Use(func(c *gin.Context) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		ctx := utils.WithLogger(c.Request.Context(), logger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	router.POST("/accounts", handler.CreateAccount)
	router.GET("/accounts/:id", handler.GetAccount)

	return router, mockService
}

func TestCreateAccount_Success(t *testing.T) {
	router, mockService := setupAccountTestRouter()

	// Mock successful account creation
	expectedAccount := &models.Account{
		ID:        "acc_12345",
		OwnerName: "John Doe",
		Balance:   1000.00,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockService.On("CreateAccount", mock.Anything, mock.MatchedBy(func(req *models.CreateAccountRequest) bool {
		return req.OwnerName == "John Doe" && req.InitialBalance == 1000.00
	})).Return(expectedAccount, nil)

	// Create request
	requestBody := models.CreateAccountRequest{
		OwnerName:      "John Doe",
		InitialBalance: 1000.00,
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/accounts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Perform request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "Account created successfully", response["message"])
	assert.Contains(t, response, "account")

	mockService.AssertExpectations(t)
}

func TestCreateAccount_InvalidJSON(t *testing.T) {
	router, _ := setupAccountTestRouter()

	req, _ := http.NewRequest("POST", "/accounts", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request body", response["error"])
}

func TestCreateAccount_InvalidBalance(t *testing.T) {
	testCases := []struct {
		name        string
		balance     float64
		expectedErr string
	}{
		{
			name:        "Negative balance",
			balance:     -100.00,
			expectedErr: "initial balance cannot be negative",
		},
		{
			name:        "Excessive amount",
			balance:     1000000000.00,
			expectedErr: "initial balance exceeds maximum allowed amount",
		},
		{
			name:        "Too many decimal places",
			balance:     100.123,
			expectedErr: "initial balance cannot have more than 2 decimal places",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := setupAccountTestRouter()

			requestBody := models.CreateAccountRequest{
				OwnerName:      "John Doe",
				InitialBalance: tc.balance,
			}
			jsonBody, _ := json.Marshal(requestBody)

			req, _ := http.NewRequest("POST", "/accounts", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, "Invalid initial balance", response["error"])
			assert.Contains(t, response["details"], tc.expectedErr)
		})
	}
}

func TestCreateAccount_ServiceError(t *testing.T) {
	router, mockService := setupAccountTestRouter()

	// Mock service error
	mockService.On("CreateAccount", mock.Anything, mock.Anything).Return(nil, errors.New("database connection failed"))

	requestBody := models.CreateAccountRequest{
		OwnerName:      "John Doe",
		InitialBalance: 1000.00,
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/accounts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "database connection failed")

	mockService.AssertExpectations(t)
}

func TestGetAccount_Success(t *testing.T) {
	router, mockService := setupAccountTestRouter()

	expectedAccount := &models.Account{
		ID:        "acc_12345",
		OwnerName: "John Doe",
		Balance:   1500.75,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}

	mockService.On("GetAccountByID", mock.Anything, "acc_12345").Return(expectedAccount, nil)

	req, _ := http.NewRequest("GET", "/accounts/acc_12345", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "account")
	account := response["account"].(map[string]interface{})
	assert.Equal(t, "acc_12345", account["id"])
	assert.Equal(t, "John Doe", account["owner_name"])
	assert.Equal(t, 1500.75, account["balance"])

	mockService.AssertExpectations(t)
}

func TestGetAccount_NotFound(t *testing.T) {
	router, mockService := setupAccountTestRouter()

	mockService.On("GetAccountByID", mock.Anything, "acc_nonexistent").Return(nil, errors.New("account not found"))

	req, _ := http.NewRequest("GET", "/accounts/acc_nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "account not found")

	mockService.AssertExpectations(t)
}

// Test validation helper functions
func TestValidateOwnerName(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{"Valid name", "John Doe", false, ""},
		{"Empty name", "", true, "owner name is required"},
		{"Whitespace only", "   ", true, "owner name is required"},
		{"Too short", "A", true, "owner name must be at least 2 characters long"},
		{"With numbers", "John123", true, "can only contain letters"},
		{"Valid hyphenated", "Mary-Jane", false, ""},
		{"Valid apostrophe", "O'Connor", false, ""},
		{"Double spaces", "John  Doe", true, "invalid character sequences"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOwnerName(tc.input)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateInitialBalance(t *testing.T) {
	testCases := []struct {
		name        string
		input       float64
		expectError bool
		errorMsg    string
	}{
		{"Valid balance", 1000.50, false, ""},
		{"Zero balance", 0.00, false, ""},
		{"Negative balance", -100.00, true, "cannot be negative"},
		{"Excessive amount", 1000000000.00, true, "exceeds maximum"},
		{"Too many decimals", 100.123, true, "more than 2 decimal places"},
		{"Valid two decimals", 99.99, false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInitialBalance(tc.input)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 5.0, abs(-5.0))
	assert.Equal(t, 5.0, abs(5.0))
	assert.Equal(t, 0.0, abs(0.0))
}
