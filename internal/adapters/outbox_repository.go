package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

const outboxEventsTable = "outbox_events"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type OutboxRepository struct {
	conn *sql.DB
}

func NewOutboxRepository(dbConn *sql.DB) *OutboxRepository {
	return &OutboxRepository{
		conn: dbConn,
	}
}

// SaveInTx saves an outbox event within a transaction
func (r *OutboxRepository) SaveInTx(tx *sql.Tx, event *domain.OutboxEvent) error {
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query, args, err := psql.Insert(outboxEventsTable).
		Columns("id", "aggregate_id", "aggregate_type", "event_type", "priority",
			"retry_count", "max_retries", "status", "payload", "created_at").
		Values(event.ID, event.AggregateID, event.AggregateType, event.EventType, event.Priority,
			event.RetryCount, event.MaxRetries, event.Status, payloadJSON, event.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	_, err = tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	return nil
}

// FindPending finds pending outbox events ordered by priority and creation time
func (r *OutboxRepository) FindPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	query, args, err := psql.Select("id", "aggregate_id", "aggregate_type", "event_type", "priority",
		"retry_count", "max_retries", "status", "payload", "error_details",
		"created_at", "published_at", "processing_started_at", "next_retry_at").
		From(outboxEventsTable).
		Where(sq.Eq{"status": "pending"}).
		OrderBy("priority DESC", "created_at ASC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := r.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox events: %w", err)
	}
	defer rows.Close()

	return r.scanOutboxEvents(rows)
}

// FindRetryable finds failed events that are ready for retry
func (r *OutboxRepository) FindRetryable(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	query, args, err := psql.Select("id", "aggregate_id", "aggregate_type", "event_type", "priority",
		"retry_count", "max_retries", "status", "payload", "error_details",
		"created_at", "published_at", "processing_started_at", "next_retry_at").
		From(outboxEventsTable).
		Where(sq.And{
			sq.Eq{"status": "failed"},
			sq.NotEq{"next_retry_at": nil},
			sq.Expr("next_retry_at <= NOW()"),
			sq.Expr("retry_count < max_retries"),
		}).
		OrderBy("next_retry_at ASC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := r.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query retryable outbox events: %w", err)
	}
	defer rows.Close()

	return r.scanOutboxEvents(rows)
}

// ClaimForProcessing atomically claims an event for processing
func (r *OutboxRepository) ClaimForProcessing(ctx context.Context, eventID string) (*domain.OutboxEvent, error) {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "processing").
		Set("processing_started_at", sq.Expr("NOW()")).
		Where(sq.And{
			sq.Eq{"id": eventID},
			sq.Or{sq.Eq{"status": "pending"}, sq.Eq{"status": "failed"}},
		}).
		Suffix("RETURNING id, aggregate_id, aggregate_type, event_type, priority, retry_count, max_retries, status, payload, error_details, created_at, published_at, processing_started_at, next_retry_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build update query: %w", err)
	}

	row := tx.QueryRowContext(ctx, query, args...)
	event, err := r.scanSingleOutboxEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "published").
		Set("published_at", sq.Expr("NOW()")).
		Set("processing_started_at", nil).
		Where(sq.Eq{"id": eventID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MarkFailed marks an event as failed with error details and retry timing
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, errorDetails string, nextRetryAt *time.Time) error {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "failed").
		Set("retry_count", sq.Expr("retry_count + 1")).
		Set("error_details", errorDetails).
		Set("next_retry_at", nextRetryAt).
		Set("processing_started_at", nil).
		Where(sq.Eq{"id": eventID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MarkPermanentlyFailed marks an event as permanently failed after max retries
func (r *OutboxRepository) MarkPermanentlyFailed(ctx context.Context, eventID string, errorDetails string) error {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "failed").
		Set("error_details", errorDetails).
		Set("next_retry_at", nil).
		Set("processing_started_at", nil).
		Where(sq.Eq{"id": eventID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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
	Scan(dest ...any) error
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
func (r *OutboxRepository) unmarshalPayload(eventType domain.OutboxEventType, payloadJSON []byte) (any, error) {
	switch eventType {
	case domain.OutboxEventAnalysisRequested, domain.OutboxEventAnalysisRetry:
		var payload domain.AnalysisRequestPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AnalysisRequestPayload: %w", err)
		}
		return payload, nil
	default:
		// For unknown event types, fallback to generic any
		var payload any
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
