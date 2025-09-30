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
	_ "github.com/lib/pq"
)

const analysisTable = "analysis"

type (
	AnalysisRepository struct {
		conn *sqlx.DB
	}

	analysisRow struct {
		ID              string         `db:"id"`
		URL             string         `db:"url"`
		Status          string         `db:"status"`
		ContentHash     sql.NullString `db:"content_hash"`
		ContentSize     sql.NullInt64  `db:"content_size"`
		CreatedAt       time.Time      `db:"created_at"`
		CompletedAt     sql.NullTime   `db:"completed_at"`
		Duration        sql.NullInt64  `db:"duration"`
		Results         sql.NullString `db:"results"`
		ErrorCode       sql.NullString `db:"error_code"`
		ErrorMessage    sql.NullString `db:"error_message"`
		ErrorStatusCode sql.NullInt32  `db:"error_status_code"`
		ErrorDetails    sql.NullString `db:"error_details"`
		LockVersion     int            `db:"lock_version"`
	}

	queryExecutor interface {
		GetContext(ctx context.Context, dest any, query string, args ...any) error
	}
)

func NewAnalysisRepository(db *sqlx.DB) *AnalysisRepository {
	return &AnalysisRepository{
		conn: db,
	}
}

func (r *AnalysisRepository) Find(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	return r.findByCriteria(
		ctx,
		sq.Eq{"id": analysisID},
		"",
		fmt.Sprintf("analysis with ID %s not found", analysisID),
	)
}

func (r *AnalysisRepository) FindByContentHash(ctx context.Context, contentHash string) (*domain.Analysis, error) {
	analysis, err := r.findByCriteria(
		ctx,
		sq.And{
			sq.Eq{"content_hash": contentHash},
			sq.Eq{"status": domain.StatusCompleted},
			sq.NotEq{"results": nil},
		},
		"completed_at DESC",
		sql.ErrNoRows.Error(),
	)
	if err != nil {
		if err.Error() == sql.ErrNoRows.Error() {
			return nil, sql.ErrNoRows
		}

		return nil, err
	}

	return analysis, nil
}

func (r *AnalysisRepository) Save(ctx context.Context, url string, _ domain.AnalysisOptions) (*domain.Analysis, error) {
	normalizedURL, err := domain.NewNormalizedURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize URL: %w", err)
	}

	version, err := r.getNextVersion(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to get next version: %w", err)
	}

	analysisID := uuid.NewSHA1(AnalysisNamespace, []byte(fmt.Sprintf("%s::v%d", normalizedURL.String(), version)))

	analysis := &domain.Analysis{
		ID:        analysisID,
		URL:       url,
		Status:    domain.StatusRequested,
		CreatedAt: time.Now(),
	}

	query, args, err := psql.Insert(analysisTable).
		Columns("id", "url", "url_normalized", "status", "version", "created_at").
		Values(analysis.ID, analysis.URL, normalizedURL.String(), analysis.Status, version, analysis.CreatedAt).
		Suffix("RETURNING created_at, lock_version").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	var result struct {
		CreatedAt   time.Time `db:"created_at"`
		LockVersion int       `db:"lock_version"`
	}

	if err := r.conn.GetContext(ctx, &result, query, args...); err != nil {
		return nil, fmt.Errorf("failed to save analysis: %w", err)
	}

	analysis.CreatedAt = result.CreatedAt
	analysis.LockVersion = result.LockVersion

	return analysis, nil
}

// SaveInTx saves an analysis within a transaction for the outbox pattern
func (r *AnalysisRepository) SaveInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	url string,
	options domain.AnalysisOptions,
) (*domain.Analysis, error) {
	normalizedURL, err := domain.NewNormalizedURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize URL: %w", err)
	}

	version, err := r.getNextVersionInTx(ctx, tx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to get next version: %w", err)
	}

	analysisID := uuid.NewSHA1(AnalysisNamespace, []byte(fmt.Sprintf("%s::v%d", normalizedURL.String(), version)))

	analysis := &domain.Analysis{
		ID:        analysisID,
		URL:       url,
		Status:    domain.StatusRequested,
		CreatedAt: time.Now(),
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	query, args, err := psql.Insert(analysisTable).
		Columns("id", "url", "url_normalized", "status", "version", "options", "created_at").
		Values(analysis.ID, analysis.URL, normalizedURL.String(), analysis.Status, version, optionsJSON, analysis.CreatedAt).
		Suffix("RETURNING created_at, lock_version").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	var result struct {
		CreatedAt   time.Time `db:"created_at"`
		LockVersion int       `db:"lock_version"`
	}

	if err := tx.Get(&result, query, args...); err != nil {
		return nil, fmt.Errorf("failed to save analysis in transaction: %w", err)
	}

	analysis.CreatedAt = result.CreatedAt
	analysis.LockVersion = result.LockVersion

	return analysis, nil
}

func (r *AnalysisRepository) Update(ctx context.Context, analysisID, contentHash string, contentSize int64, results *domain.AnalysisData) error {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	return r.updateByCriteria(
		ctx,
		psql.Update(analysisTable).
			Set("content_hash", contentHash).
			Set("content_size", contentSize).
			Set("status", domain.StatusCompleted).
			Set("results", resultsJSON).
			Set("completed_at", sq.Expr("NOW()")).
			Set("updated_at", sq.Expr("NOW()")).
			Set("lock_version", sq.Expr("lock_version + 1")).
			Where(sq.Eq{"id": analysisID}),
		"failed to update analysis results",
	)
}

func (r *AnalysisRepository) UpdateStatus(ctx context.Context, analysisID string, status domain.AnalysisStatus) error {
	return r.updateByCriteria(
		ctx,
		psql.Update(analysisTable).
			Set("status", status).
			Set("updated_at", sq.Expr("NOW()")).
			Set("lock_version", sq.Expr("lock_version + 1")).
			Where(sq.Eq{"id": analysisID}),
		"failed to update analysis status",
	)
}

func (r *AnalysisRepository) UpdateCompletionDuration(ctx context.Context, analysisID string, durationMs int64) error {
	return r.updateByCriteria(
		ctx,
		psql.Update(analysisTable).
			Set("duration", durationMs).
			Set("completed_at", sq.Expr("NOW()")).
			Set("updated_at", sq.Expr("NOW()")).
			Set("lock_version", sq.Expr("lock_version + 1")).
			Where(sq.Eq{"id": analysisID}),
		"failed to update analysis duration and completion",
	)
}

func (r *AnalysisRepository) Delete(ctx context.Context, analysisID string) error {
	tx, err := r.conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query, args, err := psql.Delete(analysisTable).
		Where(sq.Eq{"id": analysisID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete query: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete analysis: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("analysis with ID %s not found", analysisID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *AnalysisRepository) CopyResults(ctx context.Context, analysisID, sourceAnalysisID, contentHash string) error {
	var contentSize int64
	sizeQuery, sizeArgs, err := psql.Select("content_size").
		From(analysisTable).
		Where(sq.Eq{"id": sourceAnalysisID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build size query: %w", err)
	}

	if err := r.conn.GetContext(ctx, &contentSize, sizeQuery, sizeArgs...); err != nil {
		return fmt.Errorf("failed to get content size from source analysis: %w", err)
	}

	return r.updateByCriteria(
		ctx,
		psql.Update(analysisTable).
			Set("content_hash", contentHash).
			Set("content_size", contentSize).
			Set("status", domain.StatusCompleted).
			Set("results", sq.Expr("(SELECT results FROM analysis WHERE id = ?)", sourceAnalysisID)).
			Set("completed_at", sq.Expr("NOW()")).
			Where(sq.Eq{"id": analysisID}),
		"failed to copy analysis results",
	)
}

func (r *AnalysisRepository) MarkFailed(ctx context.Context, analysisID, errorCode, errorMessage string, statusCode int) error {
	return r.updateByCriteria(
		ctx,
		psql.Update(analysisTable).
			Set("status", domain.StatusFailed).
			Set("error_code", errorCode).
			Set("error_message", errorMessage).
			Set("error_status_code", statusCode).
			Set("completed_at", sq.Expr("NOW()")).
			Set("updated_at", sq.Expr("NOW()")).
			Set("lock_version", sq.Expr("lock_version + 1")).
			Where(sq.Eq{"id": analysisID}),
		"failed to mark analysis as failed",
	)
}

func (r *AnalysisRepository) getNextVersion(ctx context.Context, url string) (int, error) {
	return r.getNextVersionWithExecutor(ctx, r.conn, url)
}

func (r *AnalysisRepository) getNextVersionInTx(ctx context.Context, tx *sqlx.Tx, url string) (int, error) {
	return r.getNextVersionWithExecutor(ctx, tx, url)
}

func (r *AnalysisRepository) getNextVersionWithExecutor(ctx context.Context, exec queryExecutor, url string) (int, error) {
	query, args, err := psql.Select("COALESCE(MAX(version), 0) + 1 as next_version").
		From(analysisTable).
		Where(sq.Eq{"url": url}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("failed to build version query: %w", err)
	}

	var nextVersion int
	if err := exec.GetContext(ctx, &nextVersion, query, args...); err != nil {
		return 0, fmt.Errorf("failed to get next version: %w", err)
	}

	return nextVersion, nil
}

func (r *AnalysisRepository) findByCriteria(
	ctx context.Context,
	criteria sq.Sqlizer,
	orderBy string,
	errorContext string,
) (*domain.Analysis, error) {
	queryBuilder := psql.Select("id", "url", "status", "content_hash", "content_size", "created_at",
		"completed_at", "duration", "results", "error_code", "error_message", "error_status_code", "error_details", "lock_version").
		From(analysisTable).
		Where(criteria)

	if orderBy != "" {
		queryBuilder = queryBuilder.OrderBy(orderBy).Limit(1)
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	var row analysisRow
	if err := r.conn.GetContext(ctx, &row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%s", errorContext)
		}

		return nil, fmt.Errorf("failed to query analysis: %w", err)
	}

	return r.convertRowToAnalysis(row)
}

func (r *AnalysisRepository) convertRowToAnalysis(row analysisRow) (*domain.Analysis, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse id: %w", err)
	}

	analysis := &domain.Analysis{
		ID:          id,
		URL:         row.URL,
		Status:      domain.AnalysisStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		LockVersion: row.LockVersion,
	}

	if row.ContentHash.Valid {
		analysis.ContentHash = row.ContentHash.String
	}

	if row.ContentSize.Valid {
		analysis.ContentSize = row.ContentSize.Int64
	}

	if row.CompletedAt.Valid {
		analysis.CompletedAt = &row.CompletedAt.Time
	}

	if row.Duration.Valid {
		duration := time.Duration(row.Duration.Int64) * time.Millisecond
		analysis.Duration = &duration
	}

	if row.Results.Valid {
		var results domain.AnalysisData
		if err := json.Unmarshal([]byte(row.Results.String), &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results JSON: %w", err)
		}
		analysis.Results = &results
	}

	if row.ErrorCode.Valid {
		analysisError := &domain.AnalysisError{
			Code:    row.ErrorCode.String,
			Message: row.ErrorMessage.String,
		}

		if row.ErrorStatusCode.Valid {
			analysisError.StatusCode = int(row.ErrorStatusCode.Int32)
		}

		if row.ErrorDetails.Valid {
			analysisError.Details = row.ErrorDetails.String
		}

		analysis.Error = analysisError
	}

	return analysis, nil
}

func (r *AnalysisRepository) updateByCriteria(
	ctx context.Context,
	updateBuilder sq.UpdateBuilder,
	errorContext string,
) error {
	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	_, err = r.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", errorContext, err)
	}

	return nil
}
