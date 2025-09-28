package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type PostgresRepository struct {
	storageClient *infrastructure.Storage
}

func NewPostgresRepository(storageClient *infrastructure.Storage) PostgresRepository {
	return PostgresRepository{
		storageClient: storageClient,
	}
}

func normalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Ensure scheme is present
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	}

	// Convert scheme and host to lowercase
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	// Remove default ports
	if (parsedURL.Scheme == "http" && strings.HasSuffix(parsedURL.Host, ":80")) ||
		(parsedURL.Scheme == "https" && strings.HasSuffix(parsedURL.Host, ":443")) {
		parsedURL.Host = parsedURL.Host[:strings.LastIndex(parsedURL.Host, ":")]
	}

	// Remove trailing slash if path is just "/"
	if parsedURL.Path == "/" {
		parsedURL.Path = ""
	}

	// Preserve query parameters as they affect content
	// Query parameters are kept as-is since they determine what content is served

	// Remove fragment since it's client-side only and doesn't affect server content
	parsedURL.Fragment = ""

	return parsedURL.String(), nil
}

func (r PostgresRepository) Find(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		SELECT id, url, status, created_at, completed_at, duration_ms, results,
		       error_code, error_message, error_status_code, error_details
		FROM analysis
		WHERE id = $1
	`

	var analysis domain.Analysis
	var completedAt sql.NullTime
	var durationMs sql.NullInt64
	var resultsJSON sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var errorStatusCode sql.NullInt32
	var errorDetails sql.NullString

	err = db.QueryRowContext(ctx, query, analysisID).Scan(
		&analysis.ID,
		&analysis.URL,
		&analysis.Status,
		&analysis.CreatedAt,
		&completedAt,
		&durationMs,
		&resultsJSON,
		&errorCode,
		&errorMessage,
		&errorStatusCode,
		&errorDetails,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("analysis with ID %s not found", analysisID)
		}
		return nil, fmt.Errorf("failed to query analysis: %w", err)
	}

	if completedAt.Valid {
		analysis.CompletedAt = &completedAt.Time
	}

	if durationMs.Valid {
		duration := time.Duration(durationMs.Int64) * time.Millisecond
		analysis.Duration = &duration
	}

	if resultsJSON.Valid {
		var results domain.AnalysisData
		if err := json.Unmarshal([]byte(resultsJSON.String), &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results JSON: %w", err)
		}
		analysis.Results = &results
	}

	if errorCode.Valid {
		analysisError := &domain.AnalysisError{
			Code:    errorCode.String,
			Message: errorMessage.String,
		}
		if errorStatusCode.Valid {
			analysisError.StatusCode = int(errorStatusCode.Int32)
		}
		if errorDetails.Valid {
			analysisError.Details = errorDetails.String
		}
		analysis.Error = analysisError
	}

	return &analysis, nil
}

func (r PostgresRepository) Save(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error) {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	// Normalize the URL
	normalizedURL, err := normalizeURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize URL: %w", err)
	}

	// Create new analysis from parameters
	analysis := &domain.Analysis{
		ID:        uuid.New(),
		URL:       url,
		Status:    domain.StatusRequested,
		CreatedAt: time.Now(),
	}

	query := `
		INSERT INTO analysis (
			id, url, url_normalized, status, created_at
		) VALUES (
			$1, $2, $3, $4, $5
		)
		RETURNING id, created_at
	`

	err = db.QueryRowContext(ctx, query,
		analysis.ID,
		analysis.URL,
		normalizedURL,
		analysis.Status,
		analysis.CreatedAt,
	).Scan(&analysis.ID, &analysis.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to save analysis: %w", err)
	}

	return analysis, nil
}

func (r PostgresRepository) Update(ctx context.Context, url string, options domain.AnalysisOptions) error {
	// This method signature doesn't make sense for updating an analysis
	// We need the analysis ID to update, but the interface only provides url and options
	// This appears to be a design issue with the interface
	return fmt.Errorf("update method requires analysis ID but interface only provides url and options")
}

// UpdateAnalysis updates an existing analysis record
func (r PostgresRepository) UpdateAnalysis(ctx context.Context, analysis *domain.Analysis) error {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	var resultsJSON sql.NullString
	if analysis.Results != nil {
		resultsBytes, err := json.Marshal(analysis.Results)
		if err != nil {
			return fmt.Errorf("failed to marshal results: %w", err)
		}
		resultsJSON = sql.NullString{String: string(resultsBytes), Valid: true}
	}

	var completedAt sql.NullTime
	if analysis.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *analysis.CompletedAt, Valid: true}
	}

	var durationMs sql.NullInt64
	if analysis.Duration != nil {
		durationMs = sql.NullInt64{Int64: analysis.Duration.Milliseconds(), Valid: true}
	}

	var errorCode, errorMessage, errorDetails sql.NullString
	var errorStatusCode sql.NullInt32
	if analysis.Error != nil {
		errorCode = sql.NullString{String: analysis.Error.Code, Valid: true}
		errorMessage = sql.NullString{String: analysis.Error.Message, Valid: true}
		if analysis.Error.StatusCode != 0 {
			errorStatusCode = sql.NullInt32{Int32: int32(analysis.Error.StatusCode), Valid: true}
		}
		if analysis.Error.Details != "" {
			errorDetails = sql.NullString{String: analysis.Error.Details, Valid: true}
		}
	}

	query := `
		UPDATE analysis SET
			status = $2,
			completed_at = $3,
			duration_ms = $4,
			results = $5,
			error_code = $6,
			error_message = $7,
			error_status_code = $8,
			error_details = $9
		WHERE id = $1
	`

	result, err := db.ExecContext(ctx, query,
		analysis.ID,
		analysis.Status,
		completedAt,
		durationMs,
		resultsJSON,
		errorCode,
		errorMessage,
		errorStatusCode,
		errorDetails,
	)

	if err != nil {
		return fmt.Errorf("failed to update analysis: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("analysis with ID %s not found", analysis.ID)
	}

	return nil
}

func (r PostgresRepository) Delete(ctx context.Context, analysisID string) error {
	db, err := r.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `DELETE FROM analysis WHERE id = $1`

	result, err := db.ExecContext(ctx, query, analysisID)
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

	return nil
}
