package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	StatusRequested  AnalysisStatus = "requested"
	StatusInProgress AnalysisStatus = "in_progress"
	StatusCompleted  AnalysisStatus = "completed"
	StatusFailed     AnalysisStatus = "failed"

	HTML5   HTMLVersion = "HTML5"
	HTML401 HTMLVersion = "HTML 4.01"
	XHTML10 HTMLVersion = "XHTML 1.0"
	XHTML11 HTMLVersion = "XHTML 1.1"
	Unknown HTMLVersion = "Unknown"

	LinkTypeInternal LinkType = "internal"
	LinkTypeExternal LinkType = "external"

	InputTypePassword = "password"

	EventTypeStarted   Event = "analysis_started"
	EventTypeProgress  Event = "analysis_progress"
	EventTypeCompleted Event = "analysis_completed"
	EventTypeFailed    Event = "analysis_failed"

	// Outbox status constants
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusPublished  OutboxStatus = "published"
	OutboxStatusFailed     OutboxStatus = "failed"

	// Priority constants
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"

	// Outbox event types
	OutboxEventAnalysisRequested OutboxEventType = "analysis.requested"
	OutboxEventAnalysisRetry     OutboxEventType = "analysis.retry"
)

type (
	AnalysisStatus  string
	HTMLVersion     string
	LinkType        string
	FormMethod      string
	Event           string
	OutboxStatus    string
	Priority        string
	OutboxEventType string

	Analysis struct {
		ID          uuid.UUID      `json:"analysis_id"`
		URL         string         `json:"url"`
		Status      AnalysisStatus `json:"status"`
		CreatedAt   time.Time      `json:"created_at"`
		CompletedAt *time.Time     `json:"completed_at,omitempty"`
		Duration    *time.Duration `json:"duration,omitempty"`
		Results     *AnalysisData  `json:"results,omitempty"`
		Error       *AnalysisError `json:"error,omitempty"`
	}

	AnalysisData struct {
		HTMLVersion   HTMLVersion   `json:"html_version"`
		Title         string        `json:"title"`
		HeadingCounts HeadingCounts `json:"heading_counts"`
		Links         LinkAnalysis  `json:"links"`
		Forms         FormAnalysis  `json:"forms"`
	}

	HeadingCounts struct {
		H1 int `json:"h1"`
		H2 int `json:"h2"`
		H3 int `json:"h3"`
		H4 int `json:"h4"`
		H5 int `json:"h5"`
		H6 int `json:"h6"`
	}

	LinkAnalysis struct {
		InternalCount     int                `json:"internal_count"`
		ExternalCount     int                `json:"external_count"`
		TotalCount        int                `json:"total_count"`
		InaccessibleLinks []InaccessibleLink `json:"inaccessible_links"`
	}

	InaccessibleLink struct {
		URL        string `json:"url"`
		StatusCode int    `json:"status_code"`
		Error      string `json:"error"`
	}

	FormAnalysis struct {
		TotalCount         int         `json:"total_count"`
		LoginFormsDetected int         `json:"login_forms_detected"`
		LoginFormDetails   []LoginForm `json:"login_form_details"`
	}

	LoginForm struct {
		Method FormMethod `json:"method"`
		Action string     `json:"action"`
		Fields []string   `json:"fields"`
	}

	AnalysisError struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		StatusCode int    `json:"status_code,omitempty"`
		Details    string `json:"details,omitempty"`
	}

	AnalysisOptions struct {
		IncludeHeadings bool          `json:"include_headings"`
		CheckLinks      bool          `json:"check_links"`
		DetectForms     bool          `json:"detect_forms"`
		Timeout         time.Duration `json:"timeout"`
	}

	WebPageAnalyzer interface {
		Analyze(ctx context.Context, url string, options AnalysisOptions) (*AnalysisData, error)
	}

	HTMLAnalyzer interface {
		ExtractHTMLVersion(html string) HTMLVersion
		ExtractTitle(html string) string
		ExtractHeadingCounts(html string) HeadingCounts
		ExtractLinks(html string, baseURL string) ([]Link, error)
		ExtractForms(html string, baseURL string) FormAnalysis
	}

	Link struct {
		URL  string
		Type LinkType
	}

	WebPageContent struct {
		URL         string
		StatusCode  int
		HTML        string
		ContentType string
		Headers     map[string]string
	}

	AnalysisEvent struct {
		Type    Event       `json:"type"`
		Data    interface{} `json:"data"`
		EventID string      `json:"event_id"`
	}

	// OutboxEvent domain models
	OutboxEvent struct {
		ID                  uuid.UUID       `json:"id"`
		AggregateID         uuid.UUID       `json:"aggregate_id"`
		AggregateType       string          `json:"aggregate_type"`
		EventType           OutboxEventType `json:"event_type"`
		Priority            Priority        `json:"priority"`
		RetryCount          int             `json:"retry_count"`
		MaxRetries          int             `json:"max_retries"`
		Status              OutboxStatus    `json:"status"`
		Payload             interface{}     `json:"payload"`
		ErrorDetails        *string         `json:"error_details,omitempty"`
		CreatedAt           time.Time       `json:"created_at"`
		PublishedAt         *time.Time      `json:"published_at,omitempty"`
		ProcessingStartedAt *time.Time      `json:"processing_started_at,omitempty"`
		NextRetryAt         *time.Time      `json:"next_retry_at,omitempty"`
	}

	// AnalysisRequestPayload represents the payload for analysis request events
	AnalysisRequestPayload struct {
		AnalysisID uuid.UUID       `json:"analysis_id"`
		URL        string          `json:"url"`
		Options    AnalysisOptions `json:"options"`
		Priority   Priority        `json:"priority"`
		CreatedAt  time.Time       `json:"created_at"`
	}
)
