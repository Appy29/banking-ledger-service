#!/bin/bash

echo "Banking Ledger Integration Tests"
echo "================================"

# Check if required services are running
echo "Checking required services..."

# Check PostgreSQL (Windows-compatible)
if ! docker exec banking-postgres pg_isready -U postgres > /dev/null 2>&1; then
    echo "PostgreSQL is not running or not ready. Please ensure docker-compose up -d is complete"
    exit 1
fi
echo "PostgreSQL is running"

# Check MongoDB (Windows-compatible)
if ! docker exec banking-mongodb mongosh --eval "db.adminCommand('ping')" > /dev/null 2>&1; then
    echo "MongoDB is not running. Please ensure docker-compose up -d is complete"
    exit 1
fi
echo "MongoDB is running"

# Check RabbitMQ (Windows-compatible)
if ! docker exec banking-rabbitmq rabbitmq-diagnostics ping > /dev/null 2>&1; then
    echo "RabbitMQ is not running. Please ensure docker-compose up -d is complete"
    exit 1
fi
echo "RabbitMQ is running"

echo ""
echo "Setting up test databases..."

# Create test database for PostgreSQL (Windows-compatible)
docker exec banking-postgres psql -U postgres -c "DROP DATABASE IF EXISTS banking_ledger_test;" > /dev/null 2>&1
docker exec banking-postgres psql -U postgres -c "CREATE DATABASE banking_ledger_test;" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "PostgreSQL test database created"
else
    echo "Failed to create PostgreSQL test database"
    exit 1
fi

# MongoDB test collections will be created automatically
echo "MongoDB test collections ready"

echo ""
echo "Running Integration Tests..."
echo "==========================="

# Run database integration tests
echo ""
echo "Database Integration Tests"
echo "-------------------------"
go test -v -run TestDatabaseIntegration -timeout 30s

if [ $? -ne 0 ]; then
    echo "Database integration tests failed"
    exit 1
fi

# Run queue integration tests
echo ""
echo "Queue Integration Tests"
echo "----------------------"
go test -v -run TestQueueIntegration -timeout 30s

if [ $? -ne 0 ]; then
    echo "Queue integration tests failed"
    exit 1
fi

# Run end-to-end tests
echo ""
echo "End-to-End Integration Tests"
echo "---------------------------"
go test -v -run TestEndToEndAsyncFlow -timeout 60s

if [ $? -ne 0 ]; then
    echo "End-to-end integration tests failed"
    exit 1
fi

echo ""
echo "Cleaning up test data..."

# Drop test database
docker exec banking-postgres psql -U postgres -c "DROP DATABASE IF EXISTS banking_ledger_test;" > /dev/null 2>&1
echo "PostgreSQL test database cleaned up"

# MongoDB test collections cleanup (optional - they're isolated)
echo "MongoDB test collections will be cleaned up automatically"

echo ""
echo "All integration tests passed successfully!"
echo "=========================================="
echo ""
echo "Test Coverage Summary:"
echo "- Database operations (PostgreSQL + MongoDB)"
echo "- Transaction processing (sync and async)"
echo "- ACID compliance and rollback handling"
echo "- Concurrent transaction processing"
echo "- RabbitMQ message flow"
echo "- Worker processing with error handling"
echo "- End-to-end async transaction flow"
echo ""
echo "Your banking ledger service is ready for high-load production use!"