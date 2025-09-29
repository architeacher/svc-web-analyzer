package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

type OutboxRepository struct {
	storageClient *infrastructure.Storage
}

func NewOutboxRepository(storageClient *infrastructure.Storage) *OutboxRepository {
	return &OutboxRepository{
		storageClient: storageClient,
	}
}

// SaveInTx saves an outbox event within a transaction
func (r *OutboxRepository) SaveInTx(tx *sql.Tx, event *domain.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (
			id, aggregate_id, aggregate_type, event_type, priority,
			retry_count, max_retries, status, payload, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = tx.Exec(
		query,
		event.ID,
		event.AggregateID,
		event.AggregateType,
		event.EventType,
		event.Priority,
		event.RetryCount,
		event.MaxRetries,
		event.Status,
		payloadJSON,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	return nil
}

// FindPending finds pending outbox events ordered by priority and creation time
func (r *OutboxRepository) FindPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_id, aggregate_type, event_type, priority,
			   retry_count, max_retries, status, payload, error_details,
			   created_at, published_at, processing_started_at, next_retry_at
		FROM outbox_events
		WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT $1`

	db, err := r.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox events: %w", err)
	}
	defer rows.Close()

	return r.scanOutboxEvents(rows)
}

// FindRetryable finds failed events that are ready for retry
func (r *OutboxRepository) FindRetryable(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_id, aggregate_type, event_type, priority,
			   retry_count, max_retries, status, payload, error_details,
			   created_at, published_at, processing_started_at, next_retry_at
		FROM outbox_events
		WHERE status = 'failed'
		  AND next_retry_at IS NOT NULL
		  AND next_retry_at <= NOW()
		  AND retry_count < max_retries
		ORDER BY next_retry_at ASC
		LIMIT $1`

	db, err := r.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query retryable outbox events: %w", err)
	}
	defer rows.Close()

	return r.scanOutboxEvents(rows)
}

// ClaimForProcessing atomically claims an event for processing
func (r *OutboxRepository) ClaimForProcessing(ctx context.Context, eventID string) (*domain.OutboxEvent, error) {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Try to claim the event atomically
	updateQuery := `
		UPDATE outbox_events
		SET status = 'processing', processing_started_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'failed')
		RETURNING id, aggregate_id, aggregate_type, event_type, priority,
				  retry_count, max_retries, status, payload, error_details,
				  created_at, published_at, processing_started_at, next_retry_at`

	row := tx.QueryRowContext(ctx, updateQuery, eventID)
	event, err := r.scanSingleOutboxEvent(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found or already claimed")
		}
		return nil, fmt.Errorf("failed to claim event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return event, nil
}

// MarkPublished marks an event as successfully published
func (r *OutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		UPDATE outbox_events
		SET status = 'published', published_at = NOW(), processing_started_at = NULL
		WHERE id = $1`

	result, err := db.ExecContext(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as published: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event not found: %s", eventID)
	}

	return nil
}

// MarkFailed marks an event as failed with error details and retry timing
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, errorDetails string, nextRetryAt *time.Time) error {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		UPDATE outbox_events
		SET status = 'failed',
			retry_count = retry_count + 1,
			error_details = $2,
			next_retry_at = $3,
			processing_started_at = NULL
		WHERE id = $1`

	result, err := db.ExecContext(ctx, query, eventID, errorDetails, nextRetryAt)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event not found: %s", eventID)
	}

	return nil
}

// MarkPermanentlyFailed marks an event as permanently failed after max retries
func (r *OutboxRepository) MarkPermanentlyFailed(ctx context.Context, eventID string, errorDetails string) error {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		UPDATE outbox_events
		SET status = 'failed',
			error_details = $2,
			next_retry_at = NULL,
			processing_started_at = NULL
		WHERE id = $1`

	result, err := db.ExecContext(ctx, query, eventID, errorDetails)
	if err != nil {
		return fmt.Errorf("failed to mark event as permanently failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event not found: %s", eventID)
	}

	return nil
}

// Helper function to scan multiple outbox events from rows
func (r *OutboxRepository) scanOutboxEvents(rows *sql.Rows) ([]*domain.OutboxEvent, error) {
	var events []*domain.OutboxEvent

	for rows.Next() {
		event, err := r.scanOutboxEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// Helper function to scan a single outbox event from a row
func (r *OutboxRepository) scanSingleOutboxEvent(row *sql.Row) (*domain.OutboxEvent, error) {
	return r.scanOutboxEventFromScanner(row)
}

// Helper function to scan outbox event from sql.Rows
func (r *OutboxRepository) scanOutboxEvent(rows *sql.Rows) (*domain.OutboxEvent, error) {
	return r.scanOutboxEventFromScanner(rows)
}

// Scanner interface for both sql.Row and sql.Rows
type scanner interface {
	Scan(dest ...interface{}) error
}

// Generic scan function that works with both sql.Row and sql.Rows
func (r *OutboxRepository) scanOutboxEventFromScanner(s scanner) (*domain.OutboxEvent, error) {
	var event domain.OutboxEvent
	var payloadJSON []byte

	err := s.Scan(
		&event.ID,
		&event.AggregateID,
		&event.AggregateType,
		&event.EventType,
		&event.Priority,
		&event.RetryCount,
		&event.MaxRetries,
		&event.Status,
		&payloadJSON,
		&event.ErrorDetails,
		&event.CreatedAt,
		&event.PublishedAt,
		&event.ProcessingStartedAt,
		&event.NextRetryAt,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal payload based on event type
	payload, err := r.unmarshalPayload(event.EventType, payloadJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	event.Payload = payload

	return &event, nil
}

// unmarshalPayload unmarshals the payload JSON into the appropriate type based on event type
func (r *OutboxRepository) unmarshalPayload(eventType domain.OutboxEventType, payloadJSON []byte) (interface{}, error) {
	switch eventType {
	case domain.OutboxEventAnalysisRequested, domain.OutboxEventAnalysisRetry:
		var payload domain.AnalysisRequestPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AnalysisRequestPayload: %w", err)
		}
		return payload, nil
	default:
		// For unknown event types, fallback to generic interface{}
		var payload interface{}
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal generic payload: %w", err)
		}
		return payload, nil
	}
}

// CalculateNextRetryTime calculates the next retry time using exponential backoff with jitter
func CalculateNextRetryTime(retryCount int, baseDelay time.Duration) time.Time {
	// Exponential backoff: baseDelay * 2^retryCount
	delay := baseDelay * time.Duration(math.Pow(2, float64(retryCount)))

	// Cap maximum delay (e.g., 30 minutes)
	maxDelay := 30 * time.Minute
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter to prevent thundering herd (up to 1 second)
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

	return time.Now().Add(delay + jitter)
}