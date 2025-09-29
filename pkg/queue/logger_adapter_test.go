package queue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLoggerAdapter_Info(t *testing.T) {
	t.Parallel()

	mockLogger := &MockInfrastructureLogger{}
	mockInfoEvent := &MockInfrastructureInfoEvent{}

	mockLogger.On("Info").Return(mockInfoEvent)

	adapter := NewLoggerAdapter(mockLogger)
	logEvent := adapter.Info()

	assert.NotNil(t, logEvent)
	mockLogger.AssertExpectations(t)
}

func TestLoggerAdapter_Error(t *testing.T) {
	t.Parallel()

	mockLogger := &MockInfrastructureLogger{}
	mockErrorEvent := &MockInfrastructureErrorEvent{}

	mockLogger.On("Error").Return(mockErrorEvent)

	adapter := NewLoggerAdapter(mockLogger)
	logEvent := adapter.Error()

	assert.NotNil(t, logEvent)
	mockLogger.AssertExpectations(t)
}

func TestLoggerAdapter_Debug(t *testing.T) {
	t.Parallel()

	mockLogger := &MockInfrastructureLogger{}
	mockDebugEvent := &MockInfrastructureDebugEvent{}

	mockLogger.On("Debug").Return(mockDebugEvent)

	adapter := NewLoggerAdapter(mockLogger)
	logEvent := adapter.Debug()

	assert.NotNil(t, logEvent)
	mockLogger.AssertExpectations(t)
}

func TestLogEventAdapter_Msg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupMock func() *LogEventAdapter
	}{
		{
			name: "info event",
			setupMock: func() *LogEventAdapter {
				mockInfoEvent := &MockInfoEvent{}
				mockInfoEvent.On("Msg", "test message")
				return &LogEventAdapter{infoEvent: mockInfoEvent}
			},
		},
		{
			name: "error event",
			setupMock: func() *LogEventAdapter {
				mockErrorEvent := &MockErrorEvent{}
				mockErrorEvent.On("Msg", "error message")
				return &LogEventAdapter{errorEvent: mockErrorEvent}
			},
		},
		{
			name: "debug event",
			setupMock: func() *LogEventAdapter {
				mockDebugEvent := &MockDebugEvent{}
				mockDebugEvent.On("Msg", "debug message")
				return &LogEventAdapter{debugEvent: mockDebugEvent}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adapter := tt.setupMock()

			if tt.name == "info event" {
				adapter.Msg("test message")
				adapter.infoEvent.(*MockInfoEvent).AssertExpectations(t)
			} else if tt.name == "error event" {
				adapter.Msg("error message")
				adapter.errorEvent.(*MockErrorEvent).AssertExpectations(t)
			} else if tt.name == "debug event" {
				adapter.Msg("debug message")
				adapter.debugEvent.(*MockDebugEvent).AssertExpectations(t)
			}
		})
	}
}

func TestLogEventAdapter_Err(t *testing.T) {
	t.Parallel()

	mockErrorEvent := &MockErrorEvent{}
	testError := errors.New("test error")

	// Mock the Err method to return another error event
	returnedErrorEvent := &MockErrorEvent{}
	mockErrorEvent.On("Err", testError).Return(returnedErrorEvent)

	adapter := &LogEventAdapter{errorEvent: mockErrorEvent}
	result := adapter.Err(testError)

	assert.NotNil(t, result)
	assert.Equal(t, returnedErrorEvent, result.(*LogEventAdapter).errorEvent)
	mockErrorEvent.AssertExpectations(t)
}

func TestLogEventAdapter_Str(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func() *LogEventAdapter
		key       string
		value     string
	}{
		{
			name: "info event str",
			setupMock: func() *LogEventAdapter {
				mockInfoEvent := &MockInfoEvent{}
				returnedInfoEvent := &MockInfoEvent{}
				mockInfoEvent.On("Str", "key", "value").Return(returnedInfoEvent)
				return &LogEventAdapter{infoEvent: mockInfoEvent}
			},
			key:   "key",
			value: "value",
		},
		{
			name: "error event str",
			setupMock: func() *LogEventAdapter {
				mockErrorEvent := &MockErrorEvent{}
				returnedErrorEvent := &MockErrorEvent{}
				mockErrorEvent.On("Str", "error_key", "error_value").Return(returnedErrorEvent)
				return &LogEventAdapter{errorEvent: mockErrorEvent}
			},
			key:   "error_key",
			value: "error_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adapter := tt.setupMock()
			result := adapter.Str(tt.key, tt.value)

			assert.NotNil(t, result)

			if tt.name == "info event str" {
				adapter.infoEvent.(*MockInfoEvent).AssertExpectations(t)
			} else if tt.name == "error event str" {
				adapter.errorEvent.(*MockErrorEvent).AssertExpectations(t)
			}
		})
	}
}

func TestLogEventAdapter_Msg_InfoEvent(t *testing.T) {
	t.Parallel()

	mockEvent := &MockInfrastructureInfoEvent{}
	mockEvent.On("Msg", "test message")

	adapter := &LogEventAdapter{infoEvent: mockEvent}
	adapter.Msg("test message")

	mockEvent.AssertExpectations(t)
}

func TestLogEventAdapter_Str_InfoEvent(t *testing.T) {
	t.Parallel()

	mockEvent := &MockInfrastructureInfoEvent{}
	returnedEvent := &MockInfrastructureInfoEvent{}
	mockEvent.On("Str", "key", "value").Return(returnedEvent)

	adapter := &LogEventAdapter{infoEvent: mockEvent}
	result := adapter.Str("key", "value")

	assert.NotNil(t, result)
	assert.Equal(t, returnedEvent, result.(*LogEventAdapter).event)
	mockEvent.AssertExpectations(t)
}

func TestLogEventAdapter_Msg_ErrorEvent(t *testing.T) {
	t.Parallel()

	mockEvent := &MockInfrastructureErrorEvent{}
	mockEvent.On("Msg", "error message")

	adapter := &LogEventAdapter{errorEvent: mockEvent}
	adapter.Msg("error message")

	mockEvent.AssertExpectations(t)
}

func TestLogEventAdapter_Err_ErrorEvent(t *testing.T) {
	t.Parallel()

	mockEvent := &MockInfrastructureErrorEvent{}
	testError := errors.New("test error")
	returnedEvent := &MockInfrastructureErrorEvent{}
	mockEvent.On("Err", testError).Return(returnedEvent)

	adapter := &LogEventAdapter{errorEvent: mockEvent}
	result := adapter.Err(testError)

	assert.NotNil(t, result)
	assert.Equal(t, returnedEvent, result.(*LogEventAdapter).event)
	mockEvent.AssertExpectations(t)
}

func TestLogEventAdapter_Str_ErrorEvent(t *testing.T) {
	t.Parallel()

	mockEvent := &MockInfrastructureErrorEvent{}
	returnedEvent := &MockInfrastructureErrorEvent{}
	mockEvent.On("Str", "key", "value").Return(returnedEvent)

	adapter := &LogEventAdapter{errorEvent: mockEvent}
	result := adapter.Str("key", "value")

	assert.NotNil(t, result)
	assert.Equal(t, returnedEvent, result.(*LogEventAdapter).event)
	mockEvent.AssertExpectations(t)
}

func TestLogEventAdapter_Msg_DebugEvent(t *testing.T) {
	t.Parallel()

	mockEvent := &MockInfrastructureDebugEvent{}
	mockEvent.On("Msg", "debug message")

	adapter := &LogEventAdapter{debugEvent: mockEvent}
	adapter.Msg("debug message")

	mockEvent.AssertExpectations(t)
}

// Mock implementations for testing

type MockInfrastructureLogger struct {
	mock.Mock
}

func (m *MockInfrastructureLogger) Info() interface{ Msg(string); Str(string, string) interface{ Msg(string) } } {
	args := m.Called()
	return args.Get(0).(interface{ Msg(string); Str(string, string) interface{ Msg(string) } })
}

func (m *MockInfrastructureLogger) Error() interface{ Msg(string); Err(error) interface{}; Str(string, string) interface{} } {
	args := m.Called()
	return args.Get(0).(interface{ Msg(string); Err(error) interface{}; Str(string, string) interface{} })
}

func (m *MockInfrastructureLogger) Debug() interface{ Msg(string) } {
	args := m.Called()
	return args.Get(0).(interface{ Msg(string) })
}

type MockInfrastructureInfoEvent struct {
	mock.Mock
}

func (m *MockInfrastructureInfoEvent) Msg(msg string) {
	m.Called(msg)
}

func (m *MockInfrastructureInfoEvent) Str(key, value string) interface{ Msg(string) } {
	args := m.Called(key, value)
	return args.Get(0).(interface{ Msg(string) })
}

func (m *MockInfrastructureInfoEvent) Err(err error) interface{ Msg(string) } {
	args := m.Called(err)
	return args.Get(0).(interface{ Msg(string) })
}

type MockInfrastructureErrorEvent struct {
	mock.Mock
}

func (m *MockInfrastructureErrorEvent) Msg(msg string) {
	m.Called(msg)
}

func (m *MockInfrastructureErrorEvent) Err(err error) interface{} {
	args := m.Called(err)
	return args.Get(0)
}

func (m *MockInfrastructureErrorEvent) Str(key, value string) interface{} {
	args := m.Called(key, value)
	return args.Get(0)
}

type MockInfrastructureDebugEvent struct {
	mock.Mock
}

func (m *MockInfrastructureDebugEvent) Msg(msg string) {
	m.Called(msg)
}

type MockInfoEvent struct {
	mock.Mock
}

func (m *MockInfoEvent) Msg(msg string) {
	m.Called(msg)
}

func (m *MockInfoEvent) Str(key, value string) interface{} {
	args := m.Called(key, value)
	return args.Get(0)
}

type MockErrorEvent struct {
	mock.Mock
}

func (m *MockErrorEvent) Msg(msg string) {
	m.Called(msg)
}

func (m *MockErrorEvent) Err(err error) interface{} {
	args := m.Called(err)
	return args.Get(0)
}

func (m *MockErrorEvent) Str(key, value string) interface{} {
	args := m.Called(key, value)
	return args.Get(0)
}

type MockDebugEvent struct {
	mock.Mock
}

func (m *MockDebugEvent) Msg(msg string) {
	m.Called(msg)
}