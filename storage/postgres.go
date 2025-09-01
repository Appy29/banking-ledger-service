package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/appy29/banking-ledger-service/models"
	_ "github.com/lib/pq"
)

type PostgresAccountStorage struct {
	db *sql.DB
}

func NewPostgresAccountStorage(dsn string) (*PostgresAccountStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create accounts table if not exists
	if err := createAccountsTable(db); err != nil {
		return nil, fmt.Errorf("failed to create accounts table: %w", err)
	}

	return &PostgresAccountStorage{db: db}, nil
}

func createAccountsTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS accounts (
		id VARCHAR(255) PRIMARY KEY,
		owner_name VARCHAR(255) NOT NULL,
		balance DECIMAL(15,2) NOT NULL DEFAULT 0.00,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_accounts_owner ON accounts(owner_name);
	`
	_, err := db.Exec(query)
	return err
}

func (s *PostgresAccountStorage) CreateAccount(ctx context.Context, account *models.Account) error {
	query := `
		INSERT INTO accounts (id, owner_name, balance, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.db.ExecContext(ctx, query,
		account.ID,
		account.OwnerName,
		account.Balance,
		account.CreatedAt,
		account.UpdatedAt,
	)
	return err
}

func (s *PostgresAccountStorage) GetAccountByID(ctx context.Context, accountID string) (*models.Account, error) {
	query := `
		SELECT id, owner_name, balance, created_at, updated_at
		FROM accounts WHERE id = $1
	`

	account := &models.Account{}
	err := s.db.QueryRowContext(ctx, query, accountID).Scan(
		&account.ID,
		&account.OwnerName,
		&account.Balance,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return account, nil
}

func (s *PostgresAccountStorage) UpdateBalance(ctx context.Context, accountID string, newBalance float64) error {
	query := `
		UPDATE accounts 
		SET balance = $1, updated_at = $2
		WHERE id = $3
	`
	result, err := s.db.ExecContext(ctx, query, newBalance, time.Now(), accountID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

// AtomicBalanceUpdate performs atomic balance updates with proper locking
func (s *PostgresAccountStorage) AtomicBalanceUpdate(ctx context.Context, accountID, transactionType string, amount float64) (float64, float64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if tx.Commit() succeeds

	// Lock the account row and get current balance
	var previousBalance float64
	err = tx.QueryRowContext(ctx, "SELECT balance FROM accounts WHERE id = $1 FOR UPDATE", accountID).Scan(&previousBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, fmt.Errorf("account not found")
		}
		return 0, 0, fmt.Errorf("failed to lock account: %w", err)
	}

	// Calculate new balance based on transaction type
	var newBalance float64
	switch transactionType {
	case "deposit":
		newBalance = previousBalance + amount
	case "withdraw":
		if previousBalance < amount {
			return 0, 0, fmt.Errorf("insufficient funds: current balance %.2f, requested %.2f", previousBalance, amount)
		}
		newBalance = previousBalance - amount
	default:
		return 0, 0, fmt.Errorf("invalid transaction type: %s", transactionType)
	}

	// Update balance atomically
	_, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = $1, updated_at = $2 WHERE id = $3",
		newBalance, time.Now(), accountID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to update balance: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return previousBalance, newBalance, nil
}

func (s *PostgresAccountStorage) Close() error {
	return s.db.Close()
}
