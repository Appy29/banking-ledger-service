package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidateAccountID validates account ID parameter
func ValidateAccountID() gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID := c.Param("id")
		if accountID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "account ID is required",
				"field": "id",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(accountID, "acc_") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid account ID format",
				"field": "id",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateTransactionID validates transaction ID parameter
func ValidateTransactionID() gin.HandlerFunc {
	return func(c *gin.Context) {
		transactionID := c.Param("id")
		if transactionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "transaction ID is required",
				"field": "id",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(transactionID, "txn_") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid transaction ID format",
				"field": "id",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidatePagination validates pagination query parameters
func ValidatePagination() gin.HandlerFunc {
	return func(c *gin.Context) {
		page := 1
		limit := 10

		if pageStr := c.Query("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err != nil || p < 1 {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "page must be a positive integer",
					"field": "page",
				})
				c.Abort()
				return
			} else {
				page = p
			}
		}

		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err != nil || l < 1 || l > 100 {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "limit must be between 1 and 100",
					"field": "limit",
				})
				c.Abort()
				return
			} else {
				limit = l
			}
		}

		c.Set("page", page)
		c.Set("limit", limit)
		c.Next()
	}
}

// ValidateJSON ensures request has valid JSON content type
func ValidateJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "POST" && c.Request.Method != "PUT" && c.Request.Method != "PATCH" {
			c.Next()
			return
		}

		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Content-Type must be application/json",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
