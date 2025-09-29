package queue

import (
	"time"
)

type connectionOptions struct {
	timeout        *time.Duration
	reconnectDelay *time.Duration
	logger         Logger
}

type connectionOption func(options *connectionOptions)

// WithLogger returns a connectionOption which sets the logger when a connection is created.
func WithLogger(l Logger) connectionOption {
	return func(o *connectionOptions) {
		o.logger = l
	}
}

// WithConnectionTimeout returns a connectionOption which sets the timeout used when establishing a connection.
func WithConnectionTimeout(timeout time.Duration) connectionOption {
	return func(o *connectionOptions) {
		o.timeout = &timeout
	}
}

// WithReconnectDelay returns a connectionOption which sets the delay between reconnection attempts.
func WithReconnectDelay(delay time.Duration) connectionOption {
	return func(o *connectionOptions) {
		o.reconnectDelay = &delay
	}
}

// publisherOptions configure a NewPublisher call. publisherOptions are set by the publisherOption
// values passed to NewPublisher.
type publisherOptions struct {
	timeout time.Duration
}

type publisherOption func(options *publisherOptions)

const (
	publishingTimeout = 3 * time.Second
)

// WithPublishingTimeout returns a publisherOption which sets the timeout used when
// publishing the message.
func WithPublishingTimeout(d time.Duration) publisherOption {
	return func(o *publisherOptions) {
		o.timeout = d
	}
}

func defaultPublisherOptions() publisherOptions {
	return publisherOptions{
		timeout: publishingTimeout,
	}
}

type consumerOptions struct {
	errHandler func(error)
	logger     Logger
}

type consumerOption func(*consumerOptions)

// WithErrorHandler returns a consumerOption which sets a handler for errors that occur when consuming messages.
func WithErrorHandler(handler func(error)) consumerOption {
	return func(o *consumerOptions) {
		o.errHandler = handler
	}
}

// WithConsumingLogger returns a consumerOption which sets the logger when consuming messages.
func WithConsumingLogger(logger Logger) consumerOption {
	return func(o *consumerOptions) {
		o.logger = logger
	}
}

func defaultConsumerOptions() consumerOptions {
	return consumerOptions{
		errHandler: func(_ error) {},
	}
}