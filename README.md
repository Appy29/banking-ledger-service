# Banking Ledger Service

A high-performance banking ledger service built with Go, featuring asynchronous transaction processing capabilities designed to handle high-load scenarios while maintaining ACID compliance.

## Architecture Overview

The service implements a microservices architecture with the following processing flow:

```
HTTP Request → API Gateway (Nginx) → Banking Service → RabbitMQ Queue → Worker Pool → Database Layer
                                            ↓
                                    Immediate Response (202 Accepted)
```

### Core Components

- **API Gateway**: Nginx reverse proxy with rate limiting and load balancing
- **Banking Service**: Go application handling business logic and HTTP endpoints
- **Message Queue**: RabbitMQ for asynchronous transaction processing
- **Worker Pool**: Background goroutines processing queued transactions
- **Data Layer**: PostgreSQL for account balances, MongoDB for transaction logs
- **Monitoring**: Health checks, processing mode indicators, and Swagger documentation

## File Structure

```
banking-ledger-service/
├── config/
│   ├── config.go           # Configuration management and environment loading
│   └── .env               # Environment variables
├── handlers/
│   ├── account.go         # Account-related HTTP handlers
│   ├── health.go          # Health and readiness check handlers
│   └── trans.go           # Transaction processing handlers
├── services/
│   ├── account.go         # Account business logic
│   ├── trans.go           # Transaction business logic
│   ├── interfaces.go      # Service interfaces for dependency injection
│   ├── mock_interfaces.go # Generated mocks for testing
│   ├── account_test.go    # Account service unit tests
│   └── trans_test.go      # Transaction service unit tests
├── storage/
│   ├── postgres.go        # PostgreSQL account storage implementation
│   └── mongodb.go         # MongoDB transaction log storage
├── queue/
│   └── rabbitmq.go        # RabbitMQ integration and message handling
├── worker/
│   └── trans_worker.go    # Background worker for async transaction processing
├── middleware/
│   ├── logger.go          # Request logging and context injection
│   └── validation.go     # Request validation middleware
├── models/
│   └── models.go          # Domain models and data structures
├── utils/
│   └── logger.go          # Logging utilities and context management
├── gateway/
│   └── nginx.conf         # Nginx configuration for API gateway
├── docs/
│   └── swagger.yml        # OpenAPI 3.0 specification
├── docker-compose.yml     # Complete service orchestration
├── Dockerfile            # Banking service container configuration
├── main.go               # Application entry point
├── integration_test.go   # Database integration tests
├── queue_integration_test.go # Queue and worker integration tests
├── run_tests.sh          # Integration test runner script
└── README.md            # This documentation
```

## API Endpoints

## API Gateway Architecture

**IMPORTANT: All API requests must go through port 80 (API Gateway), not port 8080**

- **Correct**: `http://localhost/api/v1/accounts`
- **Wrong**: `http://localhost:8080/api/v1/accounts` (internal only)

The banking service runs behind an Nginx API Gateway that provides:
- Rate limiting and security headers
- Load balancing capability
- SSL termination (when configured)
- Request routing and proxy functionality

### Account Management
- `POST /api/v1/accounts` - Create new account with initial balance
- `GET /api/v1/accounts/{id}` - Retrieve account information

### Transaction Processing
- `POST /api/v1/accounts/{id}/transactions` - Process deposit or withdrawal
- `GET /api/v1/accounts/{id}/transactions` - Get transaction history (paginated)
- `GET /api/v1/transactions/{id}` - Get specific transaction details

### System Information
- `GET /health` - Basic service health check
- `GET /ready` - Comprehensive readiness check (databases, queue)
- `GET /api/v1/processing-mode` - Current processing mode and queue status

### Documentation
- `GET /swagger.yml` - OpenAPI specification
- Swagger UI available at `http://localhost:8081`

## API Usage Examples

### Account Creation
```bash
curl -X POST http://localhost/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "owner_name": "John Doe",
    "initial_balance": 1000.00
  }'
```

### Asynchronous Transaction (High Load Scenario)
```bash
curl -X POST http://localhost/api/v1/accounts/{account_id}/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "type": "deposit",
    "amount": 250.00,
    "description": "Salary deposit"
  }'

# Response (202 Accepted):
{
  "message": "Transaction queued for processing",
  "transaction_id": "txn_...",
  "status": "pending",
  "processing_mode": "async"
}
```

### Transaction Status Monitoring
```bash
# Check transaction status
curl http://localhost/api/v1/transactions/{transaction_id}

# Check account balance
curl http://localhost/api/v1/accounts/{account_id}
```

## Setup and Deployment

### Prerequisites
- Docker and Docker Compose
- Go 1.21+ (for local development)
- Git

### Quick Start
```bash
# Clone the repository
git clone <repository-url>
cd banking-ledger-service

# Start all services
docker-compose up --build -d

# Verify services are running
docker-compose ps
```

### Service Endpoints
- **Banking API**: http://localhost (through Nginx gateway)
- **Swagger UI**: http://localhost:8081
- **RabbitMQ Management**: http://localhost:15672 (admin/admin)
- **pgAdmin**: http://localhost:5050 (admin@banking.com/admin)

### Configuration

The service uses environment variables defined in `config/.env`:

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | Banking service port |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_NAME` | banking_ledger | PostgreSQL database name |
| `MONGO_URI` | mongodb://admin:admin@localhost:27017 | MongoDB connection string |
| `RABBITMQ_URL` | amqp://admin:admin@localhost:5672/ | RabbitMQ connection string |
| `WORKER_COUNT` | 5 | Number of background workers |
| `ENVIRONMENT` | development | Runtime environment |

## Testing

### Unit Tests
Run individual service unit tests:
```bash
go test ./services/... -v
go test ./handlers/... -v
```

### Integration Tests
**Important**: Integration tests require Docker services and must be run using the provided script.

```bash
# Make the script executable
chmod +x run_tests.sh

# Run all integration tests
./run_tests.sh
```

**Note**: VS Code test runner cannot execute integration tests directly as they require Docker services and specific environment setup. Always use the `run_tests.sh` script for integration testing.

### Test Coverage
- **Unit Tests**: Service layer logic with mocked dependencies
- **Integration Tests**: Database operations with real PostgreSQL and MongoDB
- **Queue Tests**: RabbitMQ message flow and worker processing
- **End-to-End Tests**: Complete async transaction lifecycle

## Processing Modes

### Synchronous Mode
- Immediate transaction processing
- Direct database updates
- Suitable for low-to-medium load scenarios
- Automatic fallback when RabbitMQ is unavailable

### Asynchronous Mode
- Queue-based transaction processing
- Immediate response with pending status
- Background worker processing
- Designed for high-load scenarios
- Horizontal scaling through worker pool adjustment

## Data Storage Strategy

### PostgreSQL (Account Data)
- Account balances with ACID compliance
- Atomic balance updates using database transactions
- Row-level locking for concurrent safety
- Optimized for consistency and financial accuracy

### MongoDB (Transaction Logs)
- Complete transaction history and audit trail
- Optimized for write-heavy workloads
- Indexed for efficient querying and pagination
- Designed for scalability and performance

## Monitoring and Health Checks

### Health Endpoints
- `/health` - Basic liveness check
- `/ready` - Comprehensive readiness including database connectivity

### RabbitMQ Management
- Access management UI at http://localhost:15672
- Monitor queue depth, message rates, and consumer status
- Track worker performance and error rates

### Processing Mode Detection
```bash
curl http://localhost/api/v1/processing-mode
```

## Error Handling

### Business Logic Errors
- Insufficient funds validation
- Invalid transaction types
- Account not found scenarios
- Not retried in queue processing

### System Errors
- Database connection failures
- Network timeouts
- Queue connectivity issues
- Automatically retried with exponential backoff

### Transaction Rollback
- Failed transactions trigger automatic balance rollbacks
- Maintains data consistency across storage systems
- Comprehensive error logging for audit purposes

## Scaling and Performance

### Horizontal Scaling
- Increase `WORKER_COUNT` for more concurrent processing
- Add multiple banking service instances behind nginx
- Database connection pooling handles increased load

### Performance Features
- Atomic database operations prevent race conditions
- Message acknowledgment ensures reliable processing
- Dead letter queues handle permanently failed messages
- Graceful degradation to sync mode during queue failures

## Security Considerations

### Current Implementation
- Input validation for all endpoints
- SQL injection prevention through parameterized queries
- Rate limiting via nginx configuration
- Request ID tracking for audit trails


## Troubleshooting

### Common Issues

**Service Won't Start**
```bash
# Check Docker services
docker-compose ps

# Check service logs
docker-compose logs banking-api
docker-compose logs postgres
docker-compose logs mongodb
docker-compose logs rabbitmq
```

**Integration Tests Failing**
- Ensure all Docker services are running: `docker-compose up -d`
- Use the test script: `./run_tests.sh`
- Do not run integration tests through VS Code test runner
- Check database connectivity: `docker-compose logs postgres mongodb`

**Queue Processing Issues**
```bash
# Check RabbitMQ status
curl -u admin:admin http://localhost:15672/api/overview

# Monitor queue depth
# Access RabbitMQ Management UI at http://localhost:15672

# Check worker logs
docker-compose logs banking-api | grep -i worker
```

**Database Connection Problems**
```bash
# PostgreSQL connectivity
docker exec banking-postgres pg_isready -U postgres

# MongoDB connectivity
docker exec banking-mongodb mongosh --eval "db.adminCommand('ping')"

# Check network connectivity
docker network ls
docker network inspect banking-ledger-service_banking-network
```

**API Gateway Issues**
```bash
# Test nginx configuration
docker-compose exec api-gateway nginx -t

# Check nginx logs
docker-compose logs api-gateway

# Test backend connectivity
curl http://localhost/health
```

### Performance Tuning

**High Load Scenarios**
- Increase `WORKER_COUNT` in environment variables
- Scale banking service containers: `docker-compose up --scale banking-api=3`
- Monitor queue depth and adjust worker count accordingly
- Consider database connection pool tuning

**Memory and CPU Optimization**
- Add resource limits to Docker containers
- Monitor container resource usage
- Adjust nginx worker processes based on load

## Development Workflow

### Local Development
```bash
# Install dependencies
go mod tidy

# Run services
docker-compose up postgres mongodb rabbitmq -d

# Run the banking service locally
go run main.go
```

### Testing Workflow
```bash
# Unit tests only
go test ./services/... -v

# Integration tests (requires Docker services)
./run_tests.sh

# Specific test categories
go test -run TestDatabaseIntegration -v
go test -run TestQueueIntegration -v
```

### API Testing
Use the provided `test_async_flow.sh` script to test the complete async transaction flow:

```bash
chmod +x test_async_flow.sh
./test_async_flow.sh
```

## Production Deployment

### Docker Deployment
The service is designed for containerized deployment with Docker Compose providing:
- Service orchestration
- Network isolation
- Volume persistence
- Health monitoring
- Graceful shutdown handling

### Environment Configuration
- Update `config/.env` for production values
- Use Docker secrets for sensitive data
- Configure proper backup strategies for PostgreSQL and MongoDB
- Set up monitoring and alerting systems

### Monitoring Integration
- Add Prometheus metrics collection
- Implement distributed tracing
- Set up log aggregation (ELK stack)
- Configure alerting for queue depth and error rates

## ACID Compliance and Data Consistency

The service ensures financial data integrity through:
- Database transactions for balance updates
- Atomic operations preventing race conditions
- Transaction rollback mechanisms
- Comprehensive audit logging
- Consistent state across multiple storage systems

This implementation meets banking industry standards for reliability and data consistency while providing the scalability needed for high-volume transaction processing.