package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/utils"
)

type AccountService struct {
	storage AccountStorage
}

func NewAccountService(storage AccountStorage) *AccountService {
	return &AccountService{
		storage: storage,
	}
}

func (s *AccountService) CreateAccount(ctx context.Context, req *models.CreateAccountRequest) (*models.Account, error) {
	logger := utils.LoggerFromContext(ctx).With(slog.String("service", "account"))

	logger.Info("Starting account creation",
		slog.String("owner_name", req.OwnerName),
		slog.Float64("initial_balance", req.InitialBalance))

	// Validate request
	if req.OwnerName == "" {
		logger.Error("Validation failed: owner name is required")
		return nil, fmt.Errorf("owner name is required")
	}

	if req.InitialBalance < 0 {
		logger.Error("Validation failed: initial balance cannot be negative",
			slog.Float64("initial_balance", req.InitialBalance))
		return nil, fmt.Errorf("initial balance cannot be negative")
	}

	// Create account model
	account := &models.Account{
		ID:        models.NewAccountID(),
		OwnerName: req.OwnerName,
		Balance:   req.InitialBalance,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	logger = logger.With(slog.String("account_id", account.ID))
	logger.Info("Account model created, saving to storage")

	// Save to storage
	if err := s.storage.CreateAccount(ctx, account); err != nil {
		logger.Error("Failed to save account to storage", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	logger.Info("Account created successfully in storage")
	return account, nil
}

func (s *AccountService) GetAccountByID(ctx context.Context, accountID string) (*models.Account, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "account"),
		slog.String("account_id", accountID))

	logger.Info("Starting account retrieval")

	if accountID == "" {
		logger.Error("Validation failed: account ID is required")
		return nil, fmt.Errorf("account ID is required")
	}

	account, err := s.storage.GetAccountByID(ctx, accountID)
	if err != nil {
		logger.Error("Failed to retrieve account from storage", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	logger.Info("Account retrieved successfully",
		slog.String("owner_name", account.OwnerName),
		slog.Float64("balance", account.Balance))

	return account, nil
}

func (s *AccountService) GetAccountBalance(ctx context.Context, accountID string) (float64, error) {
	logger := utils.LoggerFromContext(ctx).With(
		slog.String("service", "account"),
		slog.String("account_id", accountID))

	logger.Info("Starting account balance retrieval")

	account, err := s.GetAccountByID(ctx, accountID)
	if err != nil {
		logger.Error("Failed to get account for balance check", slog.String("error", err.Error()))
		return 0, err
	}

	logger.Info("Account balance retrieved successfully", slog.Float64("balance", account.Balance))
	return account.Balance, nil
}
