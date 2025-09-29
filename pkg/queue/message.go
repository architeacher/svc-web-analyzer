package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultMaxRetryCount = 10
	retryCountHeader     = "x-retry-count"
)

var (
	// ErrRetryCountExceeded describes that a message has reached the maximum allowed retry count.
	ErrRetryCountExceeded = errors.New("retries count exceeded")
)

// delivery interface for testing purposes
type delivery interface {
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
	Reject(requeue bool) error
	GetHeaders() amqp.Table
	GetBody() []byte
}

// amqpDeliveryAdapter adapts amqp.Delivery to our delivery interface
type amqpDeliveryAdapter struct {
	amqp.Delivery
}

func (a *amqpDeliveryAdapter) GetHeaders() amqp.Table {
	return a.Headers
}

func (a *amqpDeliveryAdapter) GetBody() []byte {
	return a.Body
}

// NewAmqpDeliveryAdapter creates a new adapter for amqp.Delivery
func NewAmqpDeliveryAdapter(d amqp.Delivery) delivery {
	return &amqpDeliveryAdapter{Delivery: d}
}

// Message represents a message that can be published or consumed.
type Message struct {
	Body any `json:"body"`

	amqpDelivery delivery
}

func (m *Message) marshal() ([]byte, error) {
	content, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("could not marshal message: %w", err)
	}

	return content, nil
}

// Unmarshal parses the body field of the receiver message and stores the result in the value pointed to by target.
func (m *Message) Unmarshal(target any) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	bodyData, err := json.Marshal(m.Body)
	if err != nil {
		return fmt.Errorf("could not marshal message body: %w", err)
	}

	if err := json.Unmarshal(bodyData, target); err != nil {
		return fmt.Errorf("could not unmarshal into target: %w", err)
	}

	return nil
}

// RetryCount returns the current number of retries for the receiver message.
func (m *Message) RetryCount() (int, error) {
	headers := m.amqpDelivery.GetHeaders()
	val, ok := headers[retryCountHeader]
	if !ok {
		return 0, nil // No retry count header means first attempt
	}

	strVal, ok := val.(string)
	if !ok {
		return 0, errors.New("custom header 'x-retry-count' does not contain a string")
	}

	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, errors.New("could not convert value to integer")
	}

	return intVal, nil
}

// MsgController controls the positive or negative acknowledgement of consumed messages.
type MsgController struct {
	ch        channel
	topicName string
}

// Ack is used to positively acknowledge a consumed message.
func (ctrl *MsgController) Ack(m Message) error {
	return m.amqpDelivery.Ack(false)
}

// Nack is used to negatively acknowledge a consumed message.
func (ctrl *MsgController) Nack(m Message) error {
	return m.amqpDelivery.Nack(false, false)
}

// Reject is used to negatively acknowledge a consumed message. It will not be requeued.
func (ctrl *MsgController) Reject(m Message) error {
	return m.amqpDelivery.Reject(false)
}

// Requeue is used to re-queue a message to its original topic immediately.
func (ctrl *MsgController) Requeue(m Message) error {
	retryCount, err := m.RetryCount()
	if err != nil {
		return fmt.Errorf("failed to get retry count: %w", err)
	}
	if retryCount > defaultMaxRetryCount {
		return ErrRetryCountExceeded
	}

	body, err := m.marshal()
	if err != nil {
		return err
	}

	err = ctrl.ch.publish(
		ctrl.topicName, // exchange name
		ctrl.topicName, // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			Body: body,
			Headers: amqp.Table{
				retryCountHeader: strconv.Itoa(retryCount + 1),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to re-publish message: %w", err)
	}

	if err := m.amqpDelivery.Ack(false); err != nil {
		return fmt.Errorf("failed to ack the message: %w", err)
	}

	return nil
}