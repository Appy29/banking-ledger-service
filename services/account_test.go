package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestAccountService_CreateAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	req := &models.CreateAccountRequest{
		OwnerName:      "John Doe",
		InitialBalance: 500.00,
	}

	// Setup context with logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock expectation
	mockStorage.EXPECT().
		CreateAccount(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, account *models.Account) error {
			// Verify the account being created has correct values
			assert.Equal(t, "John Doe", account.OwnerName)
			assert.Equal(t, 500.00, account.Balance)
			assert.True(t, len(account.ID) > 0)
			assert.True(t, account.ID[:4] == "acc_")
			return nil
		}).
		Times(1)

	// Execute
	account, err := service.CreateAccount(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, "John Doe", account.OwnerName)
	assert.Equal(t, 500.00, account.Balance)
	assert.True(t, len(account.ID) > 0)
	assert.WithinDuration(t, time.Now(), account.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), account.UpdatedAt, time.Second)
}

func TestAccountService_CreateAccount_EmptyOwnerName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	req := &models.CreateAccountRequest{
		OwnerName:      "", // Empty name
		InitialBalance: 500.00,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// No storage call should be made since validation fails
	// mockStorage.EXPECT() - no expectations

	// Execute
	account, err := service.CreateAccount(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, account)
	assert.Contains(t, err.Error(), "owner name is required")
}

func TestAccountService_CreateAccount_NegativeBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	req := &models.CreateAccountRequest{
		OwnerName:      "John Doe",
		InitialBalance: -100.00, // Negative balance
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Execute
	account, err := service.CreateAccount(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, account)
	assert.Contains(t, err.Error(), "initial balance cannot be negative")
}

func TestAccountService_CreateAccount_StorageError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	req := &models.CreateAccountRequest{
		OwnerName:      "John Doe",
		InitialBalance: 500.00,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Mock storage to return error
	mockStorage.EXPECT().
		CreateAccount(gomock.Any(), gomock.Any()).
		Return(errors.New("database connection failed")).
		Times(1)

	// Execute
	account, err := service.CreateAccount(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, account)
	assert.Contains(t, err.Error(), "failed to create account")
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestAccountService_CreateAccount_ZeroBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	req := &models.CreateAccountRequest{
		OwnerName:      "John Doe",
		InitialBalance: 0.00, // Zero balance should be allowed
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockStorage.EXPECT().
		CreateAccount(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)

	// Execute
	account, err := service.CreateAccount(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, 0.00, account.Balance)
}

func TestAccountService_GetAccountByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	accountID := "acc_12345"
	expectedAccount := &models.Account{
		ID:        accountID,
		OwnerName: "John Doe",
		Balance:   750.50,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(expectedAccount, nil).
		Times(1)

	// Execute
	account, err := service.GetAccountByID(ctx, accountID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, accountID, account.ID)
	assert.Equal(t, "John Doe", account.OwnerName)
	assert.Equal(t, 750.50, account.Balance)
}

func TestAccountService_GetAccountByID_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	// Execute
	account, err := service.GetAccountByID(ctx, "")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, account)
	assert.Contains(t, err.Error(), "account ID is required")
}

func TestAccountService_GetAccountByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	accountID := "acc_nonexistent"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(nil, errors.New("account not found")).
		Times(1)

	// Execute
	account, err := service.GetAccountByID(ctx, accountID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, account)
	assert.Contains(t, err.Error(), "failed to get account")
}

func TestAccountService_GetAccountBalance_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	accountID := "acc_12345"
	expectedAccount := &models.Account{
		ID:        accountID,
		OwnerName: "John Doe",
		Balance:   1250.75,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(expectedAccount, nil).
		Times(1)

	// Execute
	balance, err := service.GetAccountBalance(ctx, accountID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 1250.75, balance)
}

func TestAccountService_GetAccountBalance_AccountNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := NewMockAccountStorage(ctrl)
	service := NewAccountService(mockStorage)

	accountID := "acc_nonexistent"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := utils.WithLogger(context.Background(), logger)

	mockStorage.EXPECT().
		GetAccountByID(ctx, accountID).
		Return(nil, errors.New("account not found")).
		Times(1)

	// Execute
	balance, err := service.GetAccountBalance(ctx, accountID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, 0.0, balance)
	assert.Contains(t, err.Error(), "failed to get account")
}
