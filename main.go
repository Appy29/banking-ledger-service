package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/appy29/banking-ledger-service/config"
	"github.com/appy29/banking-ledger-service/handlers"
	"github.com/appy29/banking-ledger-service/middleware"
	"github.com/appy29/banking-ledger-service/queue"
	"github.com/appy29/banking-ledger-service/services"
	"github.com/appy29/banking-ledger-service/storage"
	"github.com/appy29/banking-ledger-service/worker"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize structured logger
	var logger *slog.Logger
	if cfg.Environment == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	logger.Info("Banking Ledger Service starting",
		slog.String("environment", cfg.Environment),
		slog.String("server_addr", cfg.GetServerAddr()),
		slog.Int("worker_count", cfg.WorkerCount))

	// Initialize PostgreSQL storage
	logger.Info("Connecting to PostgreSQL")
	accountStorage, err := storage.NewPostgresAccountStorage(cfg.GetPostgreSQLDSN())
	if err != nil {
		logger.Error("Failed to initialize PostgreSQL storage", slog.String("error", err.Error()))
		log.Fatalf("Failed to initialize PostgreSQL storage: %v", err)
	}
	defer accountStorage.Close()
	logger.Info("PostgreSQL connected successfully")

	// Initialize MongoDB storage
	logger.Info("Connecting to MongoDB")
	transactionStorage, err := storage.NewMongoTransactionStorage(cfg.MongoURI, cfg.MongoDB, "transaction_logs")
	if err != nil {
		logger.Error("Failed to initialize MongoDB storage", slog.String("error", err.Error()))
		log.Fatalf("Failed to initialize MongoDB storage: %v", err)
	}
	defer transactionStorage.Close()
	logger.Info("MongoDB connected successfully")

	// Initialize RabbitMQ
	logger.Info("Connecting to RabbitMQ")
	rabbitmq := queue.NewRabbitMQ(cfg.RabbitMQURL)

	rabbitConnected := false
	for i := 0; i < 10; i++ {
		if err := rabbitmq.Connect(); err != nil {
			logger.Warn("RabbitMQ connection attempt failed",
				slog.Int("attempt", i+1),
				slog.String("error", err.Error()))
			time.Sleep(time.Second * 2)
		} else {
			rabbitConnected = true
			logger.Info("RabbitMQ connected successfully")
			break
		}
	}

	if !rabbitConnected {
		logger.Warn("Failed to connect to RabbitMQ - running in synchronous mode only")
	}
	defer rabbitmq.Close()

	// Initialize services
	accountService := services.NewAccountService(accountStorage)
	transactionService := services.NewTransactionService(accountStorage, transactionStorage)

	// Start background workers if RabbitMQ is connected
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	asyncMode := rabbitConnected

	if asyncMode {
		logger.Info("Starting transaction workers", slog.Int("worker_count", cfg.WorkerCount))

		for i := 1; i <= cfg.WorkerCount; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				worker := worker.NewTransactionWorker(
					workerID,
					rabbitmq,
					transactionService,
					accountService,
				)

				if err := worker.Start(ctx); err != nil && err != context.Canceled {
					logger.Error("Worker stopped", slog.Int("worker_id", workerID), slog.String("error", err.Error()))
				}
			}(i)
		}

		logger.Info("All workers started successfully")
	} else {
		logger.Info("RabbitMQ not available - running in sync mode only")
	}

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8081", "http://localhost", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
	}))

	// Add middleware
	router.Use(middleware.AddRequestID())
	router.Use(middleware.InjectLogger(logger))
	router.Use(middleware.ValidateJSON())
	router.Use(gin.Recovery())

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(accountService, transactionService, rabbitmq)
	accountHandler := handlers.NewAccountHandler(accountService)
	transactionHandler := handlers.NewTransactionHandler(transactionService, rabbitmq, asyncMode)

	// Health check routes
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/ready", healthHandler.ReadyCheck)

	// Swagger endpoint - serve from docs directory
	router.GET("/swagger.yml", func(c *gin.Context) {
		c.File("./docs/swagger.yml")
	})

	// API v1 routes with validation middleware
	v1 := router.Group("/api/v1")
	{
		// Account routes
		v1.POST("/accounts", accountHandler.CreateAccount)
		v1.GET("/accounts/:id", middleware.ValidateAccountID(), accountHandler.GetAccount)

		// Transaction routes
		v1.POST("/accounts/:id/transactions", middleware.ValidateAccountID(), transactionHandler.ProcessTransaction)
		v1.GET("/accounts/:id/transactions", middleware.ValidateAccountID(), middleware.ValidatePagination(), transactionHandler.GetTransactions)
		v1.GET("/transactions/:id", middleware.ValidateTransactionID(), transactionHandler.GetTransaction)

		// Debug/monitoring routes
		v1.GET("/processing-mode", transactionHandler.GetProcessingMode)
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan

		logger.Info("Shutdown signal received", slog.String("signal", sig.String()))
		cancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("All workers stopped gracefully")
		case <-time.After(30 * time.Second):
			logger.Warn("Timeout waiting for workers, forcing shutdown")
		}

		logger.Info("Service shutting down")
		os.Exit(0)
	}()

	// Start the server
	logger.Info("Banking Ledger Service ready",
		slog.String("address", cfg.GetServerAddr()),
		slog.Bool("async_processing", asyncMode))

	if err := router.Run(cfg.GetServerAddr()); err != nil {
		logger.Error("Failed to start server", slog.String("error", err.Error()))
		log.Fatalf("Failed to start server: %v", err)
	}
}
