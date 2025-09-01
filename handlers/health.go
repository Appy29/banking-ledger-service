package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/appy29/banking-ledger-service/queue"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	accountService     services.AccountServiceInterface
	transactionService services.TransactionServiceInterface
	rabbitMQ           *queue.RabbitMQ
}

func NewHealthHandler(accountService services.AccountServiceInterface, transactionService services.TransactionServiceInterface, rabbitMQ *queue.RabbitMQ) *HealthHandler {
	return &HealthHandler{
		accountService:     accountService,
		transactionService: transactionService,
		rabbitMQ:           rabbitMQ,
	}
}

// HealthCheck handles GET /health - Basic liveness check
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	response := gin.H{
		"status":  "healthy",
		"service": "banking-ledger-service",
		"version": "1.0.0",
	}

	c.JSON(http.StatusOK, response)
}

// ReadyCheck handles GET /ready - Comprehensive readiness check
func (h *HealthHandler) ReadyCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	checks := make(map[string]interface{})
	allHealthy := true

	// Check PostgreSQL (via account service)
	postgresStatus := h.checkPostgresConnection(ctx)
	checks["postgres"] = gin.H{
		"status": postgresStatus.Status,
		"error":  postgresStatus.Error,
	}
	if !postgresStatus.Healthy {
		allHealthy = false
	}

	// Check MongoDB (via transaction service)
	mongoStatus := h.checkMongoConnection(ctx)
	checks["mongodb"] = gin.H{
		"status": mongoStatus.Status,
		"error":  mongoStatus.Error,
	}
	if !mongoStatus.Healthy {
		allHealthy = false
	}

	// Check RabbitMQ
	rabbitStatus := h.checkRabbitMQConnection()
	checks["rabbitmq"] = gin.H{
		"status": rabbitStatus.Status,
		"error":  rabbitStatus.Error,
	}
	// Note: RabbitMQ failure doesn't make service unhealthy since it can fallback to sync mode

	response := gin.H{
		"status":   "ready",
		"services": checks,
	}

	if allHealthy {
		c.JSON(http.StatusOK, response)
	} else {
		response["status"] = "not_ready"
		c.JSON(http.StatusServiceUnavailable, response)
	}
}

type HealthStatus struct {
	Healthy bool
	Status  string
	Error   string
}

func (h *HealthHandler) checkPostgresConnection(ctx context.Context) HealthStatus {
	_, err := h.accountService.GetAccountByID(ctx, "health_check_non_existent")

	if err != nil {
		// We expect "account not found" error - this means DB is working
		if err.Error() == "failed to get account: account not found" {
			return HealthStatus{
				Healthy: true,
				Status:  "connected",
				Error:   "",
			}
		}
		// Any other error means DB connection issue
		return HealthStatus{
			Healthy: false,
			Status:  "disconnected",
			Error:   err.Error(),
		}
	}

	// Should never reach here, but just in case
	return HealthStatus{
		Healthy: true,
		Status:  "connected",
		Error:   "",
	}
}

func (h *HealthHandler) checkMongoConnection(ctx context.Context) HealthStatus {
	// Try to get a non-existent transaction to test MongoDB connectivity
	_, err := h.transactionService.GetTransactionByID(ctx, "health_check_non_existent")

	if err != nil {
		// We expect "transaction not found" error - this means MongoDB is working
		if err.Error() == "failed to get transaction: transaction not found" {
			return HealthStatus{
				Healthy: true,
				Status:  "connected",
				Error:   "",
			}
		}
		// Any other error means MongoDB connection issue
		return HealthStatus{
			Healthy: false,
			Status:  "disconnected",
			Error:   err.Error(),
		}
	}

	// Should never reach here, but just in case
	return HealthStatus{
		Healthy: true,
		Status:  "connected",
		Error:   "",
	}
}

func (h *HealthHandler) checkRabbitMQConnection() HealthStatus {
	if h.rabbitMQ == nil {
		return HealthStatus{
			Healthy: false,
			Status:  "not_configured",
			Error:   "RabbitMQ not initialized",
		}
	}

	if h.rabbitMQ.IsConnected() {
		return HealthStatus{
			Healthy: true,
			Status:  "connected",
			Error:   "",
		}
	}

	return HealthStatus{
		Healthy: false,
		Status:  "disconnected",
		Error:   "RabbitMQ connection lost",
	}
}
