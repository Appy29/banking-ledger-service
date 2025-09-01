package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/appy29/banking-ledger-service/models"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/appy29/banking-ledger-service/utils"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	accountService services.AccountServiceInterface
}

func NewAccountHandler(accountService services.AccountServiceInterface) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

// validateOwnerName validates the owner name format
func validateOwnerName(name string) error {
	// Remove extra whitespace
	name = strings.TrimSpace(name)

	if name == "" {
		return errors.New("owner name is required")
	}

	if len(name) < 2 {
		return errors.New("owner name must be at least 2 characters long")
	}

	if len(name) > 100 {
		return errors.New("owner name cannot exceed 100 characters")
	}

	// Regex for valid names: letters, spaces, hyphens, apostrophes
	// Allows names like "John Doe", "Mary-Jane Smith", "O'Connor"
	nameRegex := regexp.MustCompile(`^[a-zA-Z\s\-'\.]+$`)
	if !nameRegex.MatchString(name) {
		return errors.New("owner name can only contain letters, spaces, hyphens, apostrophes, and periods")
	}

	// Check for excessive consecutive spaces or special characters
	if strings.Contains(name, "  ") || strings.Contains(name, "--") || strings.Contains(name, "''") {
		return errors.New("owner name contains invalid character sequences")
	}

	return nil
}

// validateInitialBalance validates the initial balance amount
func validateInitialBalance(amount float64) error {
	if amount < 0 {
		return errors.New("initial balance cannot be negative")
	}

	// Check for reasonable maximum
	if amount > 999999999.99 {
		return errors.New("initial balance exceeds maximum allowed amount")
	}

	// Check for precision
	// Use a small epsilon to handle floating point precision issues
	rounded := float64(int64(amount*100+0.5)) / 100
	if abs(amount-rounded) > 0.001 {
		return errors.New("initial balance cannot have more than 2 decimal places")
	}

	return nil
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// CreateAccount handles POST /accounts
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	logger.Info("Creating account request received")

	var req models.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid request body", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate owner name
	if err := validateOwnerName(req.OwnerName); err != nil {
		logger.Error("Owner name validation failed",
			slog.String("owner_name", req.OwnerName),
			slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid owner name",
			"details": err.Error(),
		})
		return
	}

	// Validate initial balance
	if err := validateInitialBalance(req.InitialBalance); err != nil {
		logger.Error("Initial balance validation failed",
			slog.Float64("initial_balance", req.InitialBalance),
			slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid initial balance",
			"details": err.Error(),
		})
		return
	}

	// Clean up the owner name
	req.OwnerName = strings.TrimSpace(req.OwnerName)

	logger.Info("Account creation request validated",
		slog.String("owner_name", req.OwnerName),
		slog.Float64("initial_balance", req.InitialBalance))

	account, err := h.accountService.CreateAccount(ctx, &req)
	if err != nil {
		logger.Error("Failed to create account", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("Account created successfully",
		slog.String("account_id", account.ID),
		slog.String("owner_name", account.OwnerName),
		slog.Float64("balance", account.Balance))

	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created successfully",
		"account": account,
	})
}

// GetAccount handles GET /accounts/:id
func (h *AccountHandler) GetAccount(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)
	accountID := c.Param("id")

	logger = logger.With(slog.String("account_id", accountID))
	logger.Info("Get account request received")

	account, err := h.accountService.GetAccountByID(ctx, accountID)
	if err != nil {
		logger.Error("Failed to get account", slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("Account retrieved successfully",
		slog.String("owner_name", account.OwnerName),
		slog.Float64("balance", account.Balance))

	c.JSON(http.StatusOK, gin.H{
		"account": account,
	})
}
