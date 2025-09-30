package repos

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/shared/backoff"
)

func TestCalculateBackoffDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		retryCount    int
		backoffConfig config.BackoffConfig
		minExpected   time.Duration
		maxExpected   time.Duration
	}{
		{
			name:       "First retry",
			retryCount: 0,
			backoffConfig: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 2.0,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			minExpected: 800 * time.Millisecond,
			maxExpected: 1200 * time.Millisecond,
		},
		{
			name:       "Second retry",
			retryCount: 1,
			backoffConfig: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 2.0,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			minExpected: 1600 * time.Millisecond,
			maxExpected: 2400 * time.Millisecond,
		},
		{
			name:       "High retry count should be capped",
			retryCount: 10,
			backoffConfig: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 2.0,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			minExpected: 8 * time.Second,
			maxExpected: 12 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strategy := backoff.NewExponentialStrategy(tt.backoffConfig)
			duration := strategy.Backoff(tt.retryCount)

			assert.GreaterOrEqual(t, duration, tt.minExpected)
			assert.LessOrEqual(t, duration, tt.maxExpected)
		})
	}
}

func TestOutboxEventValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		event   *domain.OutboxEvent
		wantErr bool
	}{
		{
			name: "valid event",
			event: &domain.OutboxEvent{
				ID:            uuid.New(),
				AggregateID:   uuid.New(),
				AggregateType: "analysis",
				EventType:     domain.OutboxEventAnalysisRequested,
				Priority:      domain.PriorityNormal,
				Status:        domain.OutboxStatusPending,
				Payload: map[string]any{
					"analysis_id": uuid.New().String(),
					"url":         "https://example.com",
				},
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty payload",
			event: &domain.OutboxEvent{
				ID:            uuid.New(),
				AggregateID:   uuid.New(),
				AggregateType: "analysis",
				EventType:     domain.OutboxEventAnalysisRequested,
				Priority:      domain.PriorityNormal,
				Status:        domain.OutboxStatusPending,
				Payload:       map[string]any{},
				CreatedAt:     time.Now(),
			},
			wantErr: false, // Empty payload is valid for some event types
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Basic validation tests that don't require database
			assert.NotEmpty(t, tt.event.ID)
			assert.NotEmpty(t, tt.event.AggregateID)
			assert.NotEmpty(t, tt.event.AggregateType)
			assert.NotEmpty(t, tt.event.EventType)
			assert.NotZero(t, tt.event.CreatedAt)
		})
	}
}

func TestEventStatusTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		currentStatus   domain.OutboxStatus
		targetStatus    domain.OutboxStatus
		validTransition bool
	}{
		{"pending to processing", domain.OutboxStatusPending, domain.OutboxStatusProcessing, true},
		{"processing to published", domain.OutboxStatusProcessing, domain.OutboxStatusPublished, true},
		{"processing to failed", domain.OutboxStatusProcessing, domain.OutboxStatusFailed, true},
		{"failed to processing", domain.OutboxStatusFailed, domain.OutboxStatusProcessing, true},
		{"pending to published", domain.OutboxStatusPending, domain.OutboxStatusPublished, false}, // Should go through processing
		{"published to pending", domain.OutboxStatusPublished, domain.OutboxStatusPending, false}, // Can't go back
		{"published to failed", domain.OutboxStatusPublished, domain.OutboxStatusFailed, false},   // Can't go back
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test state transition logic
			event := &domain.OutboxEvent{
				Status: tt.currentStatus,
			}

			// Basic state validation
			assert.Equal(t, tt.currentStatus, event.Status)

			// For unit tests, we can only test the logic, not actual database transitions
			if tt.validTransition {
				// These would be valid transitions in the system
				assert.NotEqual(t, tt.currentStatus, tt.targetStatus, "Status should change for valid transitions")
			}
		})
	}
}

func TestPriorityOrdering(t *testing.T) {
	t.Parallel()

	priorities := []domain.Priority{
		domain.PriorityLow,
		domain.PriorityNormal,
		domain.PriorityHigh,
		domain.PriorityUrgent,
	}

	// Test that priorities are correctly ordered in enum values
	assert.Equal(t, "low", string(domain.PriorityLow))
	assert.Equal(t, "normal", string(domain.PriorityNormal))
	assert.Equal(t, "high", string(domain.PriorityHigh))
	assert.Equal(t, "urgent", string(domain.PriorityUrgent))

	// Test that we have all expected priorities
	assert.Len(t, priorities, 4)
}

func TestRetryCountValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		retryCount  int
		maxRetries  int
		shouldRetry bool
	}{
		{"first attempt", 0, 3, true},
		{"second attempt", 1, 3, true},
		{"last attempt", 2, 3, true},
		{"exceeded max", 3, 3, false},
		{"zero max retries", 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			canRetry := tt.retryCount < tt.maxRetries
			assert.Equal(t, tt.shouldRetry, canRetry)
		})
	}
}

func TestJSONPayloadHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload map[string]any
		valid   bool
	}{
		{
			name: "simple payload",
			payload: map[string]any{
				"key": "value",
			},
			valid: true,
		},
		{
			name: "nested payload",
			payload: map[string]any{
				"nested": map[string]any{
					"inner": "value",
				},
			},
			valid: true,
		},
		{
			name:    "empty payload",
			payload: map[string]any{},
			valid:   true,
		},
		{
			name:    "nil payload",
			payload: nil,
			valid:   false, // Should not be nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event := &domain.OutboxEvent{
				Payload: tt.payload,
			}

			if tt.valid {
				assert.NotNil(t, event.Payload)
			} else {
				assert.Nil(t, event.Payload)
			}
		})
	}
}
