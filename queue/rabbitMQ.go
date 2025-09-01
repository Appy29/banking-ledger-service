// queue/rabbitmq.go
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

const (
	TransactionQueue = "transaction_queue"
	ExchangeName     = "banking_exchange"
	RoutingKey       = "transaction.process"
)

// TransactionMessage represents a transaction to be processed
type TransactionMessage struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Type      string    `json:"type"` // "deposit" or "withdraw"
	Amount    float64   `json:"amount"`
	Reference string    `json:"reference"`
	CreatedAt time.Time `json:"created_at"`
}

// RabbitMQ represents RabbitMQ connection and channel
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

// NewRabbitMQ creates a new RabbitMQ instance
func NewRabbitMQ(url string) *RabbitMQ {
	return &RabbitMQ{
		url: url,
	}
}

// Connect establishes connection to RabbitMQ
func (r *RabbitMQ) Connect() error {
	var err error

	// Retry connection logic
	for i := 0; i < 10; i++ {
		r.conn, err = amqp.Dial(r.url)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ (attempt %d/10): %v", i+1, err)
		time.Sleep(time.Second * 3)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ after retries: %v", err)
	}

	r.channel, err = r.conn.Channel()
	if err != nil {
		r.conn.Close()
		return fmt.Errorf("failed to open channel: %v", err)
	}

	return r.setupExchangeAndQueue()
}

// setupExchangeAndQueue declares exchange and queue
func (r *RabbitMQ) setupExchangeAndQueue() error {
	// Declare exchange
	err := r.channel.ExchangeDeclare(
		ExchangeName, // name
		"direct",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %v", err)
	}

	// Declare queue with dead letter exchange
	args := amqp.Table{
		"x-dead-letter-exchange": ExchangeName + "_dlx",
		"x-message-ttl":          300000, // 5 minutes
	}

	_, err = r.channel.QueueDeclare(
		TransactionQueue, // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		args,             // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %v", err)
	}

	// Bind queue to exchange
	err = r.channel.QueueBind(
		TransactionQueue, // queue name
		RoutingKey,       // routing key
		ExchangeName,     // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %v", err)
	}

	// Set QoS - process one message at a time per worker
	err = r.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %v", err)
	}

	log.Println("RabbitMQ exchange and queue setup completed")
	return nil
}

// PublishTransaction publishes a transaction message to the queue
func (r *RabbitMQ) PublishTransaction(ctx context.Context, msg TransactionMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	err = r.channel.Publish(
		ExchangeName, // exchange
		RoutingKey,   // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // make message persistent
			Timestamp:    time.Now(),
			MessageId:    msg.ID,
		})

	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	log.Printf("Published transaction message: %s", msg.ID)
	return nil
}

// ConsumeTransactions returns a channel to consume transaction messages
func (r *RabbitMQ) ConsumeTransactions() (<-chan amqp.Delivery, error) {
	msgs, err := r.channel.Consume(
		TransactionQueue, // queue
		"",               // consumer
		false,            // auto-ack (we'll ack manually)
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %v", err)
	}

	return msgs, nil
}

// Close closes the RabbitMQ connection
func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// IsConnected checks if RabbitMQ is connected
func (r *RabbitMQ) IsConnected() bool {
	return r.conn != nil && !r.conn.IsClosed()
}

func (r *RabbitMQ) PurgeQueue() error {
	_, err := r.channel.QueuePurge(TransactionQueue, false)
	return err
}

// SetQoS adjusts the Quality of Service settings for testing
func (r *RabbitMQ) SetQoS(prefetchCount int) error {
	err := r.channel.Qos(prefetchCount, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %v", err)
	}
	return nil
}
