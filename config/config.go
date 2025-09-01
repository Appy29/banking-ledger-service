package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server configuration
	ServerPort string
	ServerHost string

	// PostgreSQL configuration
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// MongoDB configuration
	MongoURI string
	MongoDB  string

	// RabbitMQ configuration
	RabbitMQURL string

	// Worker configuration
	WorkerCount int

	// Application settings
	Environment string
}

func Load() *Config {
	// Load .env file from config folder
	if err := godotenv.Load("config/.env"); err != nil {
		log.Printf("Warning: Could not load .env file from config folder: %v", err)
		log.Println("Using environment variables or defaults")
	}

	return &Config{
		// Server
		ServerPort: getEnv("SERVER_PORT", "8080"),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),

		// PostgreSQL
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "banking_ledger"),

		// MongoDB
		MongoURI: getEnv("MONGO_URI", "mongodb://admin:admin@localhost:27017"),
		MongoDB:  getEnv("MONGO_DB", "transactions"),

		// RabbitMQ
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://admin:admin@localhost:5672/"),

		// Workers
		WorkerCount: getEnvInt("WORKER_COUNT", 5),

		// Application
		Environment: getEnv("ENVIRONMENT", "development"),
	}
}

// GetServerAddr returns server address
func (c *Config) GetServerAddr() string {
	return c.ServerHost + ":" + c.ServerPort
}

// GetPostgreSQLDSN returns PostgreSQL connection string
func (c *Config) GetPostgreSQLDSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=disable"
}

// Helper function to get environment variable with default value
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// Helper function to get integer environment variable with default value
func getEnvInt(key string, defaultVal int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal := parseInt(value); intVal > 0 {
			return intVal
		}
	}
	return defaultVal
}

// Simple parseInt without importing strconv
func parseInt(s string) int {
	result := 0
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + int(char-'0')
		} else {
			return 0 // Invalid integer
		}
	}
	return result
}
