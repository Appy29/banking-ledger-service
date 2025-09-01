package middleware

import (
	"log/slog"

	"github.com/appy29/banking-ledger-service/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AddRequestID adds request ID to context and headers
func AddRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// InjectLogger injects logger with request context into gin context
func InjectLogger(baseLogger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetString("request_id")
		if requestID == "" {
			requestID = uuid.New().String()
			c.Set("request_id", requestID)
		}

		// Create context logger with request ID
		contextLogger := baseLogger.With(
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
		)

		// Add logger to request context
		ctx := utils.WithLogger(c.Request.Context(), contextLogger)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
