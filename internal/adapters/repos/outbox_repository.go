package repos

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const outboxEventsTable = "outbox_events"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type (
	OutboxRepository struct {
		conn *sqlx.DB
	}

	outboxEventRow struct {
		ID            string     `db:"id"`
		AggregateID   string     `db:"aggregate_id"`
		AggregateType string     `db:"aggregate_type"`
		EventType     string     `db:"event_type"`
		Priority      string     `db:"priority"`
		RetryCount    int        `db:"retry_count"`
		MaxRetries    int        `db:"max_retries"`
		Status        string     `db:"status"`
		Payload       []byte     `db:"payload"`
		ErrorDetails  *string    `db:"error_details"`
		CreatedAt     time.Time  `db:"created_at"`
		StartedAt     *time.Time `db:"started_at"`
		PublishedAt   *time.Time `db:"published_at"`
		ProcessedAt   *time.Time `db:"processed_at"`
		CompletedAt   *time.Time `db:"completed_at"`
		NextRetryAt   *time.Time `db:"next_retry_at"`
	}
)

func NewOutboxRepository(db *sqlx.DB) *OutboxRepository {
	return &OutboxRepository{
		conn: db,
	}
}

// SaveInTx saves an outbox event within a transaction
func (r *OutboxRepository) SaveInTx(ctx context.Context, tx *sqlx.Tx, event *domain.OutboxEvent) error {
	if event.ID == uuid.Nil {
		eventName := fmt.Sprintf("%s::%s::%d",
			event.AggregateID.String(),
			event.EventType,
			event.CreatedAt.Unix())
		event.ID = uuid.NewSHA1(OutboxNamespace, []byte(eventName))
	}

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

	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	return nil
}

// FindPending finds pending outbox events ordered by priority and creation time.
func (r *OutboxRepository) FindPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	return r.findByCriteria(
		ctx,
		sq.Eq{"status": "pending"},
		[]string{"priority DESC", "created_at ASC"},
		limit,
		"pending outbox events",
	)
}

// FindRetryable finds failed events that are ready for retry.
func (r *OutboxRepository) FindRetryable(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	return r.findByCriteria(
		ctx,
		sq.And{
			sq.Eq{"status": "failed"},
			sq.NotEq{"next_retry_at": nil},
			sq.Expr("next_retry_at <= NOW()"),
			sq.Expr("retry_count < max_retries"),
		},
		[]string{"next_retry_at ASC"},
		limit,
		"retryable outbox events",
	)
}

func (r *OutboxRepository) findByCriteria(
	ctx context.Context,
	criteria sq.Sqlizer,
	orderBy []string,
	limit int,
	errorContext string,
) ([]*domain.OutboxEvent, error) {
	query, args, err := psql.Select("id", "aggregate_id", "aggregate_type", "event_type", "priority",
		"retry_count", "max_retries", "status", "payload", "error_details",
		"created_at", "started_at", "published_at", "processed_at", "completed_at", "next_retry_at").
		From(outboxEventsTable).
		Where(criteria).
		OrderBy(orderBy...).
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	var rows []outboxEventRow
	if err := r.conn.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("failed to query %s: %w", errorContext, err)
	}

	events := make([]*domain.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		event, err := r.convertRowToEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

// ClaimForProcessing atomically claims an event for processing
func (r *OutboxRepository) ClaimForProcessing(ctx context.Context, eventID string) (*domain.OutboxEvent, error) {
	tx, err := r.conn.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "processing").
		Set("started_at", sq.Expr("NOW()")).
		Where(sq.And{
			sq.Eq{"id": eventID},
			sq.Or{sq.Eq{"status": "pending"}, sq.Eq{"status": "failed"}},
		}).
		Suffix("RETURNING id, aggregate_id, aggregate_type, event_type, priority, retry_count, max_retries, status, payload, error_details, created_at, started_at, published_at, processed_at, completed_at, next_retry_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build update query: %w", err)
	}

	var row outboxEventRow
	if err := tx.GetContext(ctx, &row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("event not found or already claimed")
		}

		return nil, fmt.Errorf("failed to claim event: %w", err)
	}

	event, err := r.convertRowToEvent(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return event, nil
}

// MarkPublished marks an event as successfully published.
func (r *OutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	tx, err := r.conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "published").
		Set("published_at", sq.Expr("NOW()")).
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

// MarkProcessed marks when a Subscriber starts processing the analysis.
func (r *OutboxRepository) MarkProcessed(ctx context.Context, eventID string) error {
	query, args, err := psql.Update(outboxEventsTable).
		Set("processed_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": eventID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := r.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
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

// MarkCompleted marks when Subscriber completes the analysis.
func (r *OutboxRepository) MarkCompleted(ctx context.Context, eventID string) error {
	query, args, err := psql.Update(outboxEventsTable).
		Set("completed_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": eventID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := r.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark event as completed: %w", err)
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

// MarkFailed marks an event as failed with error details and retry timing.
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, errorDetails string, nextRetryAt *time.Time) error {
	tx, err := r.conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "failed").
		Set("retry_count", sq.Expr("retry_count + 1")).
		Set("error_details", errorDetails).
		Set("next_retry_at", nextRetryAt).
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

// MarkPermanentlyFailed marks an event as permanently failed after max retries.
func (r *OutboxRepository) MarkPermanentlyFailed(ctx context.Context, eventID string, errorDetails string) error {
	tx, err := r.conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query, args, err := psql.Update(outboxEventsTable).
		Set("status", "failed").
		Set("error_details", errorDetails).
		Set("next_retry_at", nil).
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

// convertRowToEvent converts a single database row to a domain event.
func (r *OutboxRepository) convertRowToEvent(row outboxEventRow) (*domain.OutboxEvent, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse id: %w", err)
	}

	aggregateID, err := uuid.Parse(row.AggregateID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse aggregate_id: %w", err)
	}

	payload, err := r.unmarshalPayload(domain.OutboxEventType(row.EventType), row.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return &domain.OutboxEvent{
		ID:            id,
		AggregateID:   aggregateID,
		AggregateType: row.AggregateType,
		EventType:     domain.OutboxEventType(row.EventType),
		Priority:      domain.Priority(row.Priority),
		RetryCount:    row.RetryCount,
		MaxRetries:    row.MaxRetries,
		Status:        domain.OutboxStatus(row.Status),
		Payload:       payload,
		ErrorDetails:  row.ErrorDetails,
		CreatedAt:     row.CreatedAt,
		StartedAt:     row.StartedAt,
		PublishedAt:   row.PublishedAt,
		ProcessedAt:   row.ProcessedAt,
		CompletedAt:   row.CompletedAt,
		NextRetryAt:   row.NextRetryAt,
	}, nil
}

func (r *OutboxRepository) GetByAggregateID(ctx context.Context, aggregateID string) (*domain.OutboxEvent, error) {
	query, args, err := psql.Select("id", "aggregate_id", "aggregate_type", "event_type", "priority",
		"retry_count", "max_retries", "status", "payload", "error_details",
		"created_at", "started_at", "published_at", "processed_at", "completed_at", "next_retry_at").
		From(outboxEventsTable).
		Where(sq.Eq{"aggregate_id": aggregateID}).
		OrderBy("created_at DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	var row outboxEventRow
	if err := r.conn.GetContext(ctx, &row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("outbox event not found for aggregate ID: %s", aggregateID)
		}

		return nil, fmt.Errorf("failed to query outbox event by aggregate ID: %w", err)
	}

	return r.convertRowToEvent(row)
}

// unmarshalPayload unmarshal the payload JSON into the appropriate type based on an event type.
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
