package adapters

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/handlers"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/commands"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/queries"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type RequestHandler struct {
	app    usecases.Application
	logger *infrastructure.Logger
}

func NewRequestHandler(
	a usecases.Application,
	logger *infrastructure.Logger,
) *RequestHandler {
	return &RequestHandler{
		app:    a,
		logger: logger,
	}
}

// AnalyzeURL implements ServerInterface.AnalyzeURL
func (h *RequestHandler) AnalyzeURL(w http.ResponseWriter, r *http.Request, params handlers.AnalyzeURLParams) {
	var req handlers.AnalyzeURLJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid request body", err.Error())
		return
	}

	if req.Url == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "bad_request", "URL is required", "url field cannot be empty")
		return
	}

	options := h.mapRequestOptionsToDomainOptions(req.Options)

	result, err := h.app.Commands.AnalyzeCommandHandler.Handle(
		r.Context(),
		commands.AnalyzeCommand{
			URL:     req.Url,
			Options: options,
		},
	)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal_server_error", "Failed to start analysis", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(result)
}

// GetAnalysis implements ServerInterface.GetAnalysis
func (h *RequestHandler) GetAnalysis(w http.ResponseWriter, r *http.Request, analysisId openapi_types.UUID, params handlers.GetAnalysisParams) {
	result, err := h.app.Queries.FetchAnalysisQueryHandler.Execute(
		r.Context(),
		queries.FetchAnalysisQuery{AnalysisID: analysisId.String()},
	)
	if err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "not_found", "Analysis not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Determine response status based on analysis state
	if result != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	} else {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
	}
}

// GetAnalysisEvents implements ServerInterface.GetAnalysisEvents
func (h *RequestHandler) GetAnalysisEvents(w http.ResponseWriter, r *http.Request, analysisId openapi_types.UUID, params handlers.GetAnalysisEventsParams) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "CacheClient-Control")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal_server_error", "Streaming not supported", "response writer does not support flushing")

		return
	}

	events, err := h.app.Queries.FetchAnalysisEventsQueryHandler.Execute(
		r.Context(),
		queries.FetchAnalysisEventsQuery{AnalysisID: analysisId.String()},
	)
	if err != nil {
		w.Write([]byte("event: error\n"))
		w.Write([]byte("data: {\"error\": \"failed to fetch events\"}\n\n"))
		flusher.Flush()

		return
	}

	flusher.Flush()

	for event := range events {
		select {
		case <-r.Context().Done():
			return
		default:
			eventData, _ := json.Marshal(event)
			w.Write([]byte("event: analysis\n"))
			w.Write([]byte("data: " + string(eventData) + "\n\n"))

			flusher.Flush()
		}
	}
}
func (h *RequestHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	readinessResult, err := h.app.Queries.FetchReadinessReportQueryHandler.Execute(
		ctx,
		queries.FetchReadinessReportQuery{},
	)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal_server_error", "Failed to check readiness", err.Error())
		return
	}

	readinessResp := handlers.ReadinessResponse{
		Status:    readinessResult.OverallStatus,
		Timestamp: now,
		Version:   stringPtr("1.0.0"),
		Checks: handlers.ReadinessResponse_Checks{
			Storage: &struct {
				Error       *string                                       `json:"error,omitempty"`
				LastChecked *time.Time                                    `json:"last_checked,omitempty"`
				Status      handlers.ReadinessResponseChecksStorageStatus `json:"status"`
			}{
				Status: handlers.ReadinessResponseChecksStorageStatus(readinessResult.Storage.Status),
			},
			Cache: &struct {
				Error       *string                                     `json:"error,omitempty"`
				LastChecked *time.Time                                  `json:"last_checked,omitempty"`
				Status      handlers.ReadinessResponseChecksCacheStatus `json:"status"`
			}{
				Status: handlers.ReadinessResponseChecksCacheStatus(readinessResult.Cache.Status),
			},
			Queue: &struct {
				Error       *string                                     `json:"error,omitempty"`
				LastChecked *time.Time                                  `json:"last_checked,omitempty"`
				Status      handlers.ReadinessResponseChecksQueueStatus `json:"status"`
			}{
				Status: handlers.ReadinessResponseChecksQueueStatus(readinessResult.Queue.Status),
			},
		},
	}

	statusCode := http.StatusOK
	if readinessResult.OverallStatus == handlers.DOWN {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(readinessResp)
}

func (h *RequestHandler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	livenessResult, err := h.app.Queries.FetchLivenessReportQueryHandler.Execute(
		ctx,
		queries.FetchLivenessReportQuery{},
	)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal_server_error", "Failed to check liveness", err.Error())
		return
	}

	livenessResp := handlers.LivenessResponse{
		Status:    livenessResult.OverallStatus,
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	statusCode := http.StatusOK
	if livenessResult.OverallStatus == handlers.LivenessResponseStatusDOWN {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(livenessResp)
}

// HealthCheck implements ServerInterface.HealthCheck
func (h *RequestHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	healthResult, err := h.app.Queries.FetchHealthReportQueryHandler.Execute(
		ctx,
		queries.FetchHealthReportQuery{},
	)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal_server_error", "Failed to check health", err.Error())
		return
	}

	healthResp := handlers.HealthResponse{
		Status:    healthResult.OverallStatus,
		Timestamp: now,
		Version:   stringPtr("1.0.0"),
		Uptime:    &healthResult.Uptime,
		Checks: handlers.HealthResponse_Checks{
			Storage: &struct {
				Details      *map[string]interface{}                    `json:"details,omitempty"`
				Error        *string                                    `json:"error,omitempty"`
				LastChecked  *time.Time                                 `json:"last_checked,omitempty"`
				ResponseTime *float32                                   `json:"response_time,omitempty"`
				Status       handlers.HealthResponseChecksStorageStatus `json:"status"`
			}{
				Status:       handlers.HealthResponseChecksStorageStatus(healthResult.Storage.Status),
				ResponseTime: &healthResult.Storage.ResponseTime,
				LastChecked:  &healthResult.Storage.LastChecked,
				Error: func() *string {
					if healthResult.Storage.Error != "" {
						return &healthResult.Storage.Error
					}

					return nil
				}(),
			},
			Cache: &struct {
				Details      *handlers.HealthResponse_Checks_Cache_Details `json:"details,omitempty"`
				Error        *string                                       `json:"error,omitempty"`
				LastChecked  *time.Time                                    `json:"last_checked,omitempty"`
				ResponseTime *float32                                      `json:"response_time,omitempty"`
				Status       handlers.HealthResponseChecksCacheStatus      `json:"status"`
			}{
				Status:       handlers.HealthResponseChecksCacheStatus(healthResult.Cache.Status),
				ResponseTime: &healthResult.Cache.ResponseTime,
				LastChecked:  &healthResult.Cache.LastChecked,
				Error: func() *string {
					if healthResult.Cache.Error != "" {
						return &healthResult.Cache.Error
					}

					return nil
				}(),
			},
			Queue: &struct {
				Details      *map[string]interface{}                  `json:"details,omitempty"`
				Error        *string                                  `json:"error,omitempty"`
				LastChecked  *time.Time                               `json:"last_checked,omitempty"`
				ResponseTime *float32                                 `json:"response_time,omitempty"`
				Status       handlers.HealthResponseChecksQueueStatus `json:"status"`
			}{
				Status:       handlers.HealthResponseChecksQueueStatus(healthResult.Queue.Status),
				ResponseTime: &healthResult.Queue.ResponseTime,
				LastChecked:  &healthResult.Queue.LastChecked,
				Error: func() *string {
					if healthResult.Queue.Error != "" {
						return &healthResult.Queue.Error
					}

					return nil
				}(),
			},
		},
	}

	statusCode := http.StatusOK
	if healthResult.OverallStatus == handlers.HealthResponseStatusDOWN {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(healthResp)
}

func stringPtr(s string) *string {
	return &s
}

func float32Ptr(f float32) *float32 {
	return &f
}

// mapRequestOptionsToDomainOptions maps HTTP request options to domain options
func (h *RequestHandler) mapRequestOptionsToDomainOptions(reqOptions *struct {
	CheckLinks      *bool `json:"check_links,omitempty"`
	DetectForms     *bool `json:"detect_forms,omitempty"`
	IncludeHeadings *bool `json:"include_headings,omitempty"`
	Timeout         *int  `json:"timeout,omitempty"`
}) domain.AnalysisOptions {
	options := domain.AnalysisOptions{
		IncludeHeadings: true,             // Default to true
		CheckLinks:      true,             // Default to true
		DetectForms:     true,             // Default to true
		Timeout:         30 * time.Second, // Default timeout
	}

	if reqOptions != nil {
		if reqOptions.IncludeHeadings != nil {
			options.IncludeHeadings = *reqOptions.IncludeHeadings
		}
		if reqOptions.CheckLinks != nil {
			options.CheckLinks = *reqOptions.CheckLinks
		}
		if reqOptions.DetectForms != nil {
			options.DetectForms = *reqOptions.DetectForms
		}
		if reqOptions.Timeout != nil {
			options.Timeout = time.Duration(*reqOptions.Timeout) * time.Second
		}
	}

	return options
}

// writeErrorResponse writes a standardized error response
func (h *RequestHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorType, message, details string) {
	errorResp := handlers.ErrorResponse{
		Error:      &errorType,
		Message:    &message,
		Details:    &details,
		StatusCode: &statusCode,
		Timestamp:  &[]time.Time{time.Now()}[0],
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResp)
}
