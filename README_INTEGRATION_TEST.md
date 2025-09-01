# Integration Testing Guide

This guide explains how to run comprehensive integration tests for the Banking Ledger Service.

## Prerequisites

### Required Services
- Docker and Docker Compose
- PostgreSQL (running on port 5432)
- MongoDB (running on port 27017)  
- RabbitMQ (running on ports 5672/15672)

### Go Dependencies
```bash
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/require
```

## Setup Instructions

### 1. Start Infrastructure Services
```bash
# Start all required services
docker-compose up -d

# Verify services are running
docker-compose ps

# Check logs if needed
docker-compose logs postgres mongodb rabbitmq
```

### 2. Verify Service Connectivity
```bash
# PostgreSQL
pg_isready -h localhost -p 5432 -U postgres

# MongoDB 
mongosh --host localhost:27017 --eval "db.adminCommand('ping')"

# RabbitMQ
curl -u admin:admin http://localhost:15672/api/overview
```

### 3. Place Test Files
Create these files in your project root:
- `integration_test.go` - Database and service integration tests
- `queue_integration_test.go` - RabbitMQ and worker tests
- `run_tests.sh` - Test runner script

## Running Tests

### Quick Run
```bash
# Make script executable
chmod +x run_tests.sh

# Run all integration tests
./run_tests.sh
```

### Manual Test Execution
```bash
# Database tests only
go test -v -run TestDatabaseIntegration -timeout 30s

# Queue tests only  
go test -v -run TestQueueIntegration -timeout 30s

# End-to-end tests
go test -v -run TestEndToEndAsyncFlow -timeout 60s

# All tests
go test -v -timeout 90s
```

## Test Coverage

### Database Integration Tests
- **Account CRUD Operations**: Create, read account data with PostgreSQL
- **Transaction Processing**: Deposit/withdrawal with dual database persistence
- **Transaction History**: Pagination and sorting with MongoDB
- **Insufficient Funds**: Business rule validation and error handling
- **Concurrent Processing**: Multiple simultaneous transactions

### Queue Integration Tests  
- **Message Publishing**: Send messages to RabbitMQ queues
- **Message Consuming**: Receive and acknowledge messages
- **Multiple Messages**: Bulk processing capabilities
- **Queue Durability**: Message persistence and delivery guarantees

### End-to-End Async Flow Tests
- **Complete Async Flow**: Pending -> Queue -> Worker -> Completed
- **Worker Error Handling**: Failed transactions with proper status updates
- **Concurrent Workers**: Multiple workers processing simultaneously
- **Status Tracking**: Real-time transaction status monitoring

### Key Scenarios Tested
1. **ACID Compliance**: Transaction rollback on storage failures
2. **High Load Handling**: Concurrent transaction processing
3. **Error Recovery**: Failed transaction handling and status updates
4. **Data Consistency**: Balance calculations across databases
5. **Message Reliability**: Queue message delivery and acknowledgment
6. **Worker Resilience**: Background processing with error handling

## Troubleshooting

### Common Issues

**PostgreSQL Connection Failed**
```bash
# Check if PostgreSQL is running
docker-compose logs postgres

# Verify connection
PGPASSWORD=postgres psql -h localhost -U postgres -l
```

**MongoDB Connection Failed**
```bash
# Check MongoDB status
docker-compose logs mongodb

# Test connection
mongosh mongodb://admin:admin@localhost:27017
```

**RabbitMQ Connection Failed**
```bash
# Check RabbitMQ status
docker-compose logs rabbitmq

# Access management UI
open http://localhost:15672 (admin/admin)
```

**Tests Hanging**
- Increase timeout values in test functions
- Check for deadlocks in concurrent tests
- Verify all resources are properly closed

### Test Data Cleanup
Tests automatically:
- Create isolated test databases
- Use separate MongoDB collections with test prefixes
- Clean up data after completion
- Handle concurrent test execution

### Performance Considerations
- Database tests may take 10-30 seconds
- Queue tests typically complete in 5-10 seconds
- End-to-end tests can take 30-60 seconds
- Concurrent tests stress-test the system under load

## Expected Results

When all tests pass, you'll have verified:
- Database operations work correctly under load
- Async transaction processing maintains data integrity
- Worker scaling handles concurrent processing
- Error conditions are handled gracefully
- The system meets banking industry reliability standards

The integration tests demonstrate that your banking ledger service can handle real-world production scenarios with high throughput and consistent data management.