package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Queue represents the main queue interface for publishing and consuming messages
type Queue interface {
	// Publisher operations
	Publish(ctx context.Context, exchange, routingKey string, payload interface{}) error
	PublishWithOptions(ctx context.Context, exchange, routingKey string, payload interface{}, opts ...publisherOption) error

	// Consumer operations
	Consume(ctx context.Context, queue, consumer string, handler MessageHandler, opts ...consumerOption) error
	StartConsumer(ctx context.Context, queue, consumer string, handler MessageHandler, opts ...consumerOption) (<-chan error, error)

	// Infrastructure operations
	DeclareExchange(name, kind string, durable, autoDelete bool) error
	DeclareQueue(name string, durable, autoDelete bool) (amqp.Queue, error)
	BindQueue(queueName, routingKey, exchangeName string) error

	// Connection management
	Connect() error
	Close() error
	IsConnected() bool
}

// MessageHandler defines the function signature for message processing
type MessageHandler func(ctx context.Context, msg Message, ctrl *MsgController) error

// RabbitMQQueue implements the Queue interface using RabbitMQ
type RabbitMQQueue struct {
	config         Config
	conn           *amqp.Connection
	channel        *ChannelWrapper
	logger         Logger
	mutex          sync.RWMutex
	reconnectDelay time.Duration
	closed         bool
}

// NewRabbitMQQueue creates a new RabbitMQ queue implementation
func NewRabbitMQQueue(config Config, opts ...connectionOption) *RabbitMQQueue {
	options := &connectionOptions{
		reconnectDelay: &[]time.Duration{5 * time.Second}[0],
	}

	for _, opt := range opts {
		opt(options)
	}

	queue := &RabbitMQQueue{
		config:         config,
		reconnectDelay: *options.reconnectDelay,
		logger:         options.logger,
	}

	return queue
}

// Connect establishes a connection to RabbitMQ
func (q *RabbitMQQueue) Connect() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.conn != nil && !q.conn.IsClosed() {
		return nil // Already connected
	}

	var err error
	q.conn, err = amqp.Dial(getURL(q.config))
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	amqpCh, err := q.conn.Channel()
	if err != nil {
		q.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}
	q.channel = &ChannelWrapper{
		amqpChan:       amqpCh,
		logger:         q.logger,
		mutex:          &sync.Mutex{},
		reconnectDelay: q.reconnectDelay,
	}

	if q.logger != nil {
		q.logger.Info().Msg("Successfully connected to RabbitMQ")
	}

	return nil
}

// Close closes the connection to RabbitMQ
func (q *RabbitMQQueue) Close() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.closed = true

	if q.channel != nil {
		q.channel.Close()
	}

	if q.conn != nil && !q.conn.IsClosed() {
		return q.conn.Close()
	}

	return nil
}

// IsConnected returns true if connected to RabbitMQ
func (q *RabbitMQQueue) IsConnected() bool {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	return q.conn != nil && !q.conn.IsClosed()
}

// DeclareExchange declares an exchange
func (q *RabbitMQQueue) DeclareExchange(name, kind string, durable, autoDelete bool) error {
	if !q.IsConnected() {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	return q.channel.ExchangeDeclare(name, kind, durable, autoDelete, false, false, nil)
}

// DeclareQueue declares a queue
func (q *RabbitMQQueue) DeclareQueue(name string, durable, autoDelete bool) (amqp.Queue, error) {
	if !q.IsConnected() {
		return amqp.Queue{}, fmt.Errorf("not connected to RabbitMQ")
	}

	return q.channel.QueueDeclare(name, durable, autoDelete, false, false, nil)
}

// BindQueue binds a queue to an exchange with a routing key
func (q *RabbitMQQueue) BindQueue(queueName, routingKey, exchangeName string) error {
	if !q.IsConnected() {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	return q.channel.QueueBind(queueName, routingKey, exchangeName, false, nil)
}

// Publish publishes a message to an exchange with default options
func (q *RabbitMQQueue) Publish(ctx context.Context, exchange, routingKey string, payload interface{}) error {
	return q.PublishWithOptions(ctx, exchange, routingKey, payload)
}

// PublishWithOptions publishes a message to an exchange with custom options
func (q *RabbitMQQueue) PublishWithOptions(ctx context.Context, exchange, routingKey string, payload interface{}, opts ...publisherOption) error {
	if !q.IsConnected() {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	options := defaultPublisherOptions()
	for _, opt := range opts {
		opt(&options)
	}

	msg := &Message{Body: payload}
	body, err := msg.marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent, // Make message persistent
		Timestamp:    time.Now(),
	}

	return q.channel.Publish(exchange, routingKey, false, false, publishing)
}

// Consume consumes messages from a queue (blocking)
func (q *RabbitMQQueue) Consume(ctx context.Context, queue, consumer string, handler MessageHandler, opts ...consumerOption) error {
	errChan, err := q.StartConsumer(ctx, queue, consumer, handler, opts...)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// StartConsumer starts consuming messages from a queue (non-blocking)
func (q *RabbitMQQueue) StartConsumer(ctx context.Context, queue, consumer string, handler MessageHandler, opts ...consumerOption) (<-chan error, error) {
	if !q.IsConnected() {
		return nil, fmt.Errorf("not connected to RabbitMQ")
	}

	options := defaultConsumerOptions()
	for _, opt := range opts {
		opt(&options)
	}

	deliveries := q.channel.Consume(queue, consumer, false, false, false, false, nil)
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		msgCtrl := &MsgController{
			ch:        q.channel,
			topicName: queue,
		}

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case delivery, ok := <-deliveries:
				if !ok {
					errChan <- fmt.Errorf("delivery channel closed")
					return
				}

				var msgData Message
				if err := json.Unmarshal(delivery.Body, &msgData); err != nil {
					if q.logger != nil {
						q.logger.Error().Err(err).Msg("failed to unmarshal message")
					}
					if options.errHandler != nil {
						options.errHandler(err)
					}
					delivery.Nack(false, false) // Reject message
					continue
				}

				msgData.amqpDelivery = NewAmqpDeliveryAdapter(delivery)
				if err := handler(ctx, msgData, msgCtrl); err != nil {
					if q.logger != nil {
						q.logger.Error().Err(err).Msg("message handler failed")
					}
					if options.errHandler != nil {
						options.errHandler(err)
					}
					continue
				}
			}
		}
	}()

	return errChan, nil
}
