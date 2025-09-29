package queue

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRabbitMQQueue_Connect(t *testing.T) {
	t.Parallel()

	config := Config{
		Scheme:   "amqp",
		Username: "test",
		Password: "test",
		Host:     "localhost",
		Port:     5672,
		Vhost:    "/",
	}

	queue := NewRabbitMQQueue(config)

	assert.NotNil(t, queue)
	assert.Equal(t, config, queue.config)
}

func TestRabbitMQQueue_Close(t *testing.T) {
	t.Parallel()

	config := Config{
		Scheme:   "amqp",
		Username: "test",
		Password: "test",
		Host:     "localhost",
		Port:     5672,
		Vhost:    "/",
	}

	queue := NewRabbitMQQueue(config)

	err := queue.Close()
	assert.NoError(t, err)

	queue.closed = true
	assert.True(t, queue.closed)
}

func TestRabbitMQQueue_IsConnected(t *testing.T) {
	t.Parallel()

	config := Config{
		Scheme:   "amqp",
		Username: "test",
		Password: "test",
		Host:     "localhost",
		Port:     5672,
		Vhost:    "/",
	}

	queue := NewRabbitMQQueue(config)

	assert.False(t, queue.IsConnected())
}

func TestRabbitMQQueue_Options(t *testing.T) {
	t.Parallel()

	config := Config{
		Scheme:   "amqp",
		Username: "test",
		Password: "test",
		Host:     "localhost",
		Port:     5672,
		Vhost:    "/",
	}

	mockLogger := &MockLogger{}

	queue := NewRabbitMQQueue(config,
		WithLogger(mockLogger),
		WithReconnectDelay(10*time.Second),
		WithConnectionTimeout(30*time.Second),
	)

	assert.NotNil(t, queue)
	assert.Equal(t, 10*time.Second, queue.reconnectDelay)
	assert.Equal(t, mockLogger, queue.logger)
}

func TestMessage_Marshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name:    "string body",
			body:    "test message",
			wantErr: false,
		},
		{
			name: "map body",
			body: map[string]any{
				"key": "value",
				"num": 42,
			},
			wantErr: false,
		},
		{
			name: "struct body",
			body: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "test",
				Age:  25,
			},
			wantErr: false,
		},
		{
			name:    "nil body",
			body:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := &Message{Body: tt.body}
			data, err := msg.marshal()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

func TestMessage_Unmarshal(t *testing.T) {
	t.Parallel()

	jsonData := `{"name":"test","age":25}`
	delivery := amqp.Delivery{
		Body: []byte(jsonData),
	}

	var bodyData map[string]any
	json.Unmarshal([]byte(jsonData), &bodyData)
	msg := Message{
		Body:         bodyData,
		amqpDelivery: NewAmqpDeliveryAdapter(delivery),
	}

	var result map[string]any
	err := msg.Unmarshal(&result)

	assert.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, float64(25), result["age"]) // JSON numbers are float64
}

func TestMsgController_Ack(t *testing.T) {
	t.Parallel()

	mockChannel := &MockChannel{}
	mockChannel.On("Close").Return(nil)

	ctrl := &MsgController{
		ch:        mockChannel,
		topicName: "test-topic",
	}

	mockDelivery := &MockDelivery{}
	mockDelivery.On("Ack", false).Return(nil)

	msg := Message{}
	msg.amqpDelivery = mockDelivery

	err := ctrl.Ack(msg)
	assert.NoError(t, err)

	mockDelivery.AssertExpectations(t)
}

func TestMsgController_Nack(t *testing.T) {
	t.Parallel()

	mockChannel := &MockChannel{}
	mockChannel.On("Close").Return(nil)

	ctrl := &MsgController{
		ch:        mockChannel,
		topicName: "test-topic",
	}

	mockDelivery := &MockDelivery{}
	mockDelivery.On("Nack", false, false).Return(nil)

	msg := Message{}
	msg.amqpDelivery = mockDelivery

	err := ctrl.Nack(msg)
	assert.NoError(t, err)

	mockDelivery.AssertExpectations(t)
}

func TestMsgController_Reject(t *testing.T) {
	t.Parallel()

	mockChannel := &MockChannel{}
	mockChannel.On("Close").Return(nil)

	ctrl := &MsgController{
		ch:        mockChannel,
		topicName: "test-topic",
	}

	mockDelivery := &MockDelivery{}
	mockDelivery.On("Reject", false).Return(nil)

	msg := Message{}
	msg.amqpDelivery = mockDelivery

	err := ctrl.Reject(msg)
	assert.NoError(t, err)

	mockDelivery.AssertExpectations(t)
}

func TestMsgController_Requeue(t *testing.T) {
	t.Parallel()

	mockChannel := &MockChannel{}
	mockChannel.On("Close").Return(nil)
	mockChannel.On("publish", "test-topic", "test-topic", false, false, mock.AnythingOfType("amqp091.Publishing")).Return(nil)

	ctrl := &MsgController{
		ch:        mockChannel,
		topicName: "test-topic",
	}

	mockDelivery := &MockDelivery{}
	mockDelivery.On("GetHeaders").Return(amqp.Table{"x-retry-count": "1"})
	mockDelivery.On("Ack", false).Return(nil)

	msg := Message{Body: map[string]any{"test": "data"}}
	msg.amqpDelivery = mockDelivery

	err := ctrl.Requeue(msg)
	assert.NoError(t, err)

	mockDelivery.AssertExpectations(t)
}

func TestChannelWrapper_Close(t *testing.T) {
	t.Parallel()

	mockChannel := &MockamqpChannel{}
	mockChannel.On("Close").Return(nil)

	wrapper := &ChannelWrapper{
		amqpChan: mockChannel,
		mutex:    &sync.Mutex{},
	}

	err := wrapper.Close()
	assert.NoError(t, err)
	assert.True(t, wrapper.isClosed())

	err = wrapper.Close()
	assert.Error(t, err)
	assert.Equal(t, amqp.ErrClosed, err)

	mockChannel.AssertExpectations(t)
}

func TestChannelWrapper_ExchangeDeclare(t *testing.T) {
	t.Parallel()

	mockChannel := &MockamqpChannel{}
	mockChannel.On("ExchangeDeclare", "test-exchange", "topic", true, false, false, false, amqp.Table(nil)).Return(nil)

	wrapper := &ChannelWrapper{
		amqpChan: mockChannel,
		mutex:    &sync.Mutex{},
	}

	err := wrapper.ExchangeDeclare("test-exchange", "topic", true, false, false, false, nil)
	assert.NoError(t, err)

	mockChannel.AssertExpectations(t)
}

func TestChannelWrapper_QueueDeclare(t *testing.T) {
	t.Parallel()

	mockChannel := &MockamqpChannel{}
	expectedQueue := amqp.Queue{
		Name:     "test-queue",
		Messages: 0,
	}
	mockChannel.On("QueueDeclare", "test-queue", true, false, false, false, amqp.Table(nil)).Return(expectedQueue, nil)

	wrapper := &ChannelWrapper{
		amqpChan: mockChannel,
		mutex:    &sync.Mutex{},
	}

	queue, err := wrapper.QueueDeclare("test-queue", true, false, false, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedQueue, queue)

	mockChannel.AssertExpectations(t)
}

func TestChannelWrapper_Publish(t *testing.T) {
	t.Parallel()

	mockChannel := &MockamqpChannel{}
	mockChannel.On("Publish", "test-exchange", "test.key", false, false, mock.AnythingOfType("amqp091.Publishing")).Return(nil)

	wrapper := &ChannelWrapper{
		amqpChan: mockChannel,
		mutex:    &sync.Mutex{},
	}

	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(`{"test": "data"}`),
	}

	err := wrapper.Publish("test-exchange", "test.key", false, false, publishing)
	assert.NoError(t, err)

	mockChannel.AssertExpectations(t)
}

func TestGetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "basic config",
			config: Config{
				Scheme:   "amqp",
				Username: "user",
				Password: "pass",
				Host:     "localhost",
				Port:     5672,
				Vhost:    "/",
			},
			expected: "amqp://user:pass@localhost/",
		},
		{
			name: "custom vhost",
			config: Config{
				Scheme:   "amqps",
				Username: "user",
				Password: "pass",
				Host:     "rabbitmq.example.com",
				Port:     5671,
				Vhost:    "/custom",
			},
			expected: "amqps://user:pass@rabbitmq.example.com/%2Fcustom",
		},
		{
			name: "root vhost",
			config: Config{
				Scheme:   "amqp",
				Username: "guest",
				Password: "guest",
				Host:     "127.0.0.1",
				Port:     5672,
				Vhost:    "/",
			},
			expected: "amqp://127.0.0.1/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := getURL(tt.config)
			assert.Equal(t, tt.expected, url)
		})
	}
}

// Mock implementations for testing

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Error() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Debug() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

type MockLogEvent struct {
	mock.Mock
}

func (m *MockLogEvent) Msg(msg string) {
	m.Called(msg)
}

func (m *MockLogEvent) Err(err error) LogEvent {
	args := m.Called(err)
	return args.Get(0).(LogEvent)
}

func (m *MockLogEvent) Str(key, value string) LogEvent {
	args := m.Called(key, value)
	return args.Get(0).(LogEvent)
}

type MockChannel struct {
	mock.Mock
}

func (m *MockChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockChannel) exchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	callArgs := m.Called(name, kind, durable, autoDelete, internal, noWait, args)
	return callArgs.Error(0)
}

func (m *MockChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	return m.exchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (m *MockChannel) queueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	callArgs := m.Called(name, durable, autoDelete, exclusive, noWait, args)
	return callArgs.Get(0).(amqp.Queue), callArgs.Error(1)
}

func (m *MockChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return m.queueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (m *MockChannel) queueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	callArgs := m.Called(name, key, exchange, noWait, args)
	return callArgs.Error(0)
}

func (m *MockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	return m.queueBind(name, key, exchange, noWait, args)
}

func (m *MockChannel) publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	callArgs := m.Called(exchange, key, mandatory, immediate, msg)
	return callArgs.Error(0)
}

func (m *MockChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	return m.publish(exchange, key, mandatory, immediate, msg)
}

func (m *MockChannel) consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) <-chan amqp.Delivery {
	callArgs := m.Called(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	return callArgs.Get(0).(<-chan amqp.Delivery)
}

func (m *MockChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) <-chan amqp.Delivery {
	return m.consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
}

func (m *MockChannel) cancel(consumer string, noWait bool) error {
	callArgs := m.Called(consumer, noWait)
	return callArgs.Error(0)
}

func (m *MockChannel) Cancel(consumer string, noWait bool) error {
	return m.cancel(consumer, noWait)
}

type MockamqpChannel struct {
	mock.Mock
}

func (m *MockamqpChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockamqpChannel) Cancel(consumer string, noWait bool) error {
	args := m.Called(consumer, noWait)
	return args.Error(0)
}

func (m *MockamqpChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	callArgs := m.Called(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	return callArgs.Get(0).(<-chan amqp.Delivery), callArgs.Error(1)
}

func (m *MockamqpChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	callArgs := m.Called(name, kind, durable, autoDelete, internal, noWait, args)
	return callArgs.Error(0)
}

func (m *MockamqpChannel) ExchangeDeclarePassive(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	callArgs := m.Called(name, kind, durable, autoDelete, internal, noWait, args)
	return callArgs.Error(0)
}

func (m *MockamqpChannel) ExchangeDelete(name string, ifUnused, noWait bool) error {
	callArgs := m.Called(name, ifUnused, noWait)
	return callArgs.Error(0)
}

func (m *MockamqpChannel) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	args := m.Called(c)
	return args.Get(0).(chan *amqp.Error)
}

func (m *MockamqpChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	callArgs := m.Called(exchange, key, mandatory, immediate, msg)
	return callArgs.Error(0)
}

func (m *MockamqpChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	callArgs := m.Called(name, key, exchange, noWait, args)
	return callArgs.Error(0)
}

func (m *MockamqpChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	callArgs := m.Called(name, durable, autoDelete, exclusive, noWait, args)
	return callArgs.Get(0).(amqp.Queue), callArgs.Error(1)
}

func (m *MockamqpChannel) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {
	callArgs := m.Called(name, ifUnused, ifEmpty, noWait)
	return callArgs.Int(0), callArgs.Error(1)
}

func (m *MockamqpChannel) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	callArgs := m.Called(name, durable, autoDelete, exclusive, noWait, args)
	return callArgs.Get(0).(amqp.Queue), callArgs.Error(1)
}

func (m *MockamqpChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	callArgs := m.Called(prefetchCount, prefetchSize, global)
	return callArgs.Error(0)
}

type MockDelivery struct {
	mock.Mock
}

func (m *MockDelivery) Ack(multiple bool) error {
	args := m.Called(multiple)
	return args.Error(0)
}

func (m *MockDelivery) Nack(multiple, requeue bool) error {
	args := m.Called(multiple, requeue)
	return args.Error(0)
}

func (m *MockDelivery) Reject(requeue bool) error {
	args := m.Called(requeue)
	return args.Error(0)
}

func (m *MockDelivery) GetHeaders() amqp.Table {
	args := m.Called()
	return args.Get(0).(amqp.Table)
}

func (m *MockDelivery) GetBody() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}
