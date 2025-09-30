// Package queue provides a robust, production-ready RabbitMQ client implementation
// with advanced features for message publishing, consuming, and connection management.
//
// # Overview
//
// This package offers a high-level abstraction over the RabbitMQ AMQP client library,
// providing automatic reconnection, retry logic, message acknowledgment control,
// and comprehensive error handling. It's designed to be framework-agnostic and
// easily integrated into any Go application requiring reliable message queuing.
//
// # Key Features
//
//   - Automatic connection management with reconnection logic
//   - Configurable retry mechanisms with exponential backoff
//   - Thread-safe operations with proper mutex usage
//   - Flexible logging interface for integration with any logging framework
//   - Message acknowledgment control (Ack, Nack, Requeue)
//   - Exchange and queue declaration management
//   - Comprehensive error handling and propagation
//   - Production-ready channel wrapper with state management
//
// # Basic Usage
//
// Creating a new queue instance:
//
//	config := queue.Config{
//		Scheme:   "amqp",
//		Username: "guest",
//		Password: "guest",
//		Host:     "localhost",
//		Port:     5672,
//		Vhost:    "/",
//	}
//
//	q := queue.NewRabbitMQQueue(config)
//	if err := q.Connect(); err != nil {
//		log.Fatal(err)
//	}
//	defer q.Close()
//
// Publishing messages:
//
//	payload := map[string]any{
//		"id":      "123",
//		"message": "Hello, World!",
//	}
//
//	err := q.Publish(ctx, "my-exchange", "routing.key", payload)
//	if err != nil {
//		log.Printf("Failed to publish: %v", err)
//	}
//
// Consuming messages:
//
//	handler := func(ctx context.Context, msg queue.Message, ctrl *queue.MsgController) error {
//		var payload map[string]any
//		if err := msg.Unmarshal(&payload); err != nil {
//			return err
//		}
//
//		// Process the message
//		log.Printf("Received: %+v", payload)
//
//		// Acknowledge the message
//		return ctrl.Ack(msg)
//	}
//
//	err := q.Consume(ctx, "my-queue", "my-consumer", handler)
//	if err != nil {
//		log.Printf("Consumer error: %v", err)
//	}
//
// # Configuration Options
//
// The package supports various configuration options through functional options:
//
//   - WithLogger: Set a custom logger implementation
//   - WithConnectionTimeout: Configure connection timeout
//   - WithReconnectDelay: Set delay between reconnection attempts
//   - WithPublishingTimeout: Configure publishing timeout
//   - WithErrorHandler: Set custom error handling for consumers
//
// # Error Handling
//
// The package provides comprehensive error handling with proper error wrapping
// and context preservation. All errors include relevant context information
// to aid in debugging and monitoring.
//
// # Thread Safety
//
// All operations are thread-safe and can be called concurrently from multiple
// goroutines. The internal channel wrapper uses appropriate synchronization
// primitives to ensure safe concurrent access.
//
// # Production Considerations
//
//   - Always use connection pooling for high-throughput applications
//   - Configure appropriate timeouts for your use case
//   - Implement proper monitoring and alerting for queue operations
//   - Use structured logging for better observability
//   - Consider message persistence settings based on your durability requirements
//
// # Logging Integration
//
// The package defines a minimal logging interface that can be adapted to work
// with any logging framework. A LoggerAdapter is provided for easy integration
// with existing logging solutions.
//
// Example with a custom logger:
//
//	logger := &CustomLogger{} // Implement the Logger interface
//	q := queue.NewRabbitMQQueue(config, queue.WithLogger(logger))
//
// # Dependencies
//
// This package depends on the official RabbitMQ AMQP client library:
//   - github.com/rabbitmq/amqp091-go
//
// # License
//
// This package is part of the svc-web-analyzer project and follows the same
// licensing terms as the parent project.
package queue
