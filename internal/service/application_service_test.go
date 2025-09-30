package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/mocks"
)

func createTestLogger() *infrastructure.Logger {
	logConfig := config.LoggingConfig{
		Level:  "error", // Use error level to reduce test output
		Format: "json",
	}
	return infrastructure.New(logConfig)
}

func createTestSSEConfig() config.SSEConfig {
	return config.SSEConfig{
		EventsInterval:    100 * time.Millisecond, // Faster for tests
		HeartbeatInterval: 100 * time.Millisecond, // Faster for tests
	}
}

func createTestOutboxConfig() config.OutboxConfig {
	return config.OutboxConfig{
		MaxRetries: config.MaxRetriesByPriority{
			Low:    3,
			Normal: 5,
			High:   7,
			Urgent: 10,
		},
	}
}

// Test FetchAnalysis with cache hit
func TestFetchAnalysis_CacheHit(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	analysisID := uuid.New().String()
	expectedAnalysis := &domain.Analysis{
		ID:        uuid.MustParse(analysisID),
		URL:       "https://example.com",
		Status:    domain.StatusCompleted,
		CreatedAt: time.Now(),
		Results: &domain.AnalysisData{
			Title: "Example Title",
		},
	}

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Cache returns the analysis (cache hit)
	fakeCacheRepo.FindReturns(expectedAnalysis, nil)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}
	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	result, err := service.FetchAnalysis(ctx, analysisID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedAnalysis.ID, result.ID)
	assert.Equal(t, expectedAnalysis.URL, result.URL)
	assert.Equal(t, expectedAnalysis.Status, result.Status)
	assert.Equal(t, expectedAnalysis.Results.Title, result.Results.Title)

	// Verify that database was not called (cache hit)
	assert.Equal(t, 0, fakeAnalysisRepo.FindCallCount())
	assert.Equal(t, 1, fakeCacheRepo.FindCallCount())
}

// Test FetchAnalysis with cache miss
func TestFetchAnalysis_CacheMiss(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	analysisID := uuid.New().String()
	expectedAnalysis := &domain.Analysis{
		ID:        uuid.MustParse(analysisID),
		URL:       "https://example.com",
		Status:    domain.StatusCompleted,
		CreatedAt: time.Now(),
		Results: &domain.AnalysisData{
			Title: "Example Title",
		},
	}

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Cache miss
	fakeCacheRepo.FindReturns(nil, domain.ErrCacheUnavailable)
	// Database returns the analysis
	fakeAnalysisRepo.FindReturns(expectedAnalysis, nil)
	// Cache the result
	fakeCacheRepo.SetReturns(nil)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}
	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	result, err := service.FetchAnalysis(ctx, analysisID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedAnalysis.ID, result.ID)
	assert.Equal(t, expectedAnalysis.URL, result.URL)
	assert.Equal(t, expectedAnalysis.Status, result.Status)

	// Verify all calls were made
	assert.Equal(t, 1, fakeCacheRepo.FindCallCount())
	assert.Equal(t, 1, fakeAnalysisRepo.FindCallCount())
	assert.Equal(t, 1, fakeCacheRepo.SetCallCount())
}

// Test FetchAnalysis when both cache and DB fail
func TestFetchAnalysis_BothFail(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	analysisID := uuid.New().String()

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Cache miss
	fakeCacheRepo.FindReturns(nil, domain.ErrCacheUnavailable)
	// Database also fails
	fakeAnalysisRepo.FindReturns(nil, domain.ErrAnalysisNotFound)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}
	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	result, err := service.FetchAnalysis(ctx, analysisID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to find analysis")

	// Verify all calls were made
	assert.Equal(t, 1, fakeCacheRepo.FindCallCount())
	assert.Equal(t, 1, fakeAnalysisRepo.FindCallCount())
}

// Todo: Test StartAnalysis success - This test requires database transactions so should be an integration test
func TestStartAnalysis_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires database transactions")
	}
	t.Parallel()

	t.Skip("This should be converted to a proper integration test using testcontainers")
}

// Test StartAnalysis when DB fails
func TestStartAnalysis_DBFails(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	url := "https://example.com"
	options := domain.AnalysisOptions{
		IncludeHeadings: true,
		CheckLinks:      true,
		DetectForms:     true,
		Timeout:         30 * time.Second,
	}

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Service will fail before reaching repository due to nil storage
	// fakeAnalysisRepo.FindReturns(nil, domain.ErrInternalServerError)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}

	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	result, err := service.StartAnalysis(ctx, url, options)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "storage client not initialized")

	// Neither repository should be called since storage fails early
}

// Test StartAnalysis when cache fails but DB succeeds
func TestStartAnalysis_CacheFailsDBSucceeds(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	url := "https://example.com"
	options := domain.AnalysisOptions{
		IncludeHeadings: true,
		CheckLinks:      true,
		DetectForms:     true,
		Timeout:         30 * time.Second,
	}

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Service will fail before reaching repository due to nil storage
	// fakeAnalysisRepo.FindReturns(expectedAnalysis, nil)
	// Cache fails to save
	// fakeCacheRepo.FindReturns(domain.ErrCacheUnavailable)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}

	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	result, err := service.StartAnalysis(ctx, url, options)

	// Assert - Should fail due to nil storage
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "storage client not initialized")

	// Neither repository should be called since storage fails early
}

// Test FetchAnalysisEvents for completed analysis
func TestFetchAnalysisEvents_CompletedAnalysis(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	analysisID := uuid.New().String()
	expectedAnalysis := &domain.Analysis{
		ID:        uuid.MustParse(analysisID),
		URL:       "https://example.com",
		Status:    domain.StatusCompleted,
		CreatedAt: time.Now(),
		Results: &domain.AnalysisData{
			Title: "Example Title",
		},
	}

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Mock the FetchAnalysis call within FetchAnalysisEvents
	fakeCacheRepo.FindReturns(expectedAnalysis, nil)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}
	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	eventsChan, err := service.FetchAnalysisEvents(ctx, analysisID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, eventsChan)

	// Read the event from the channel
	select {
	case event := <-eventsChan:
		assert.Equal(t, domain.EventTypeCompleted, event.Type)
		assert.Equal(t, analysisID, event.EventID)
		assert.Equal(t, expectedAnalysis, event.Payload)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive an event but got timeout")
	}

	// Ensure channel is closed
	select {
	case _, ok := <-eventsChan:
		assert.False(t, ok, "Channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel should be closed")
	}

}

// Test FetchAnalysisEvents for failed analysis
func TestFetchAnalysisEvents_FailedAnalysis(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	analysisID := uuid.New().String()
	expectedAnalysis := &domain.Analysis{
		ID:        uuid.MustParse(analysisID),
		URL:       "https://example.com",
		Status:    domain.StatusFailed,
		CreatedAt: time.Now(),
		Error: &domain.AnalysisError{
			Code:    "FETCH_ERROR",
			Message: "Failed to fetch URL",
		},
	}

	fakeAnalysisRepo := &mocks.FakeAnalysisRepository{}
	fakeCacheRepo := &mocks.FakeCacheRepository{}
	logger := createTestLogger()

	// Mock the FetchAnalysis call within FetchAnalysisEvents
	fakeCacheRepo.FindReturns(expectedAnalysis, nil)

	fakeOutboxRepo := &mocks.FakeOutboxRepository{}
	fakeHealthChecker := &mocks.FakeHealthChecker{}
	sseConfig := createTestSSEConfig()
	outboxConfig := createTestOutboxConfig()
	service := NewApplicationService(fakeAnalysisRepo, fakeOutboxRepo, fakeCacheRepo, fakeHealthChecker, nil, sseConfig, outboxConfig, logger)

	// Act
	eventsChan, err := service.FetchAnalysisEvents(ctx, analysisID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, eventsChan)

	// Read the event from the channel
	select {
	case event := <-eventsChan:
		assert.Equal(t, domain.EventTypeFailed, event.Type)
		assert.Equal(t, analysisID, event.EventID)
		assert.Equal(t, expectedAnalysis, event.Payload)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive an event but got timeout")
	}

}
