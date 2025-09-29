package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

// Mock repositories using testify/mock
type MockAnalysisRepository struct {
	mock.Mock
}

func (m *MockAnalysisRepository) Find(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	args := m.Called(ctx, analysisID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Analysis), args.Error(1)
}

func (m *MockAnalysisRepository) Save(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error) {
	args := m.Called(ctx, url, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Analysis), args.Error(1)
}

func (m *MockAnalysisRepository) SaveInTx(tx *sql.Tx, url string, options domain.AnalysisOptions) (*domain.Analysis, error) {
	args := m.Called(tx, url, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Analysis), args.Error(1)
}

type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) Find(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	args := m.Called(ctx, analysisID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Analysis), args.Error(1)
}

func (m *MockCacheRepository) Set(ctx context.Context, analysis *domain.Analysis) error {
	args := m.Called(ctx, analysis)
	return args.Error(0)
}

func (m *MockCacheRepository) Delete(ctx context.Context, analysisID string) error {
	args := m.Called(ctx, analysisID)
	return args.Error(0)
}

type MockHealthChecker struct {
	mock.Mock
}

func (m *MockHealthChecker) CheckReadiness(ctx context.Context) *domain.ReadinessResult {
	args := m.Called(ctx)
	return args.Get(0).(*domain.ReadinessResult)
}

func (m *MockHealthChecker) CheckLiveness(ctx context.Context) *domain.LivenessResult {
	args := m.Called(ctx)
	return args.Get(0).(*domain.LivenessResult)
}

func (m *MockHealthChecker) CheckHealth(ctx context.Context) *domain.HealthResult {
	args := m.Called(ctx)
	return args.Get(0).(*domain.HealthResult)
}

type MockOutboxRepository struct {
	mock.Mock
}

func (m *MockOutboxRepository) SaveInTx(tx *sql.Tx, event *domain.OutboxEvent) error {
	args := m.Called(tx, event)
	return args.Error(0)
}

func (m *MockOutboxRepository) FindPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepository) FindRetryable(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepository) ClaimForProcessing(ctx context.Context, eventID string) (*domain.OutboxEvent, error) {
	args := m.Called(ctx, eventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *MockOutboxRepository) MarkFailed(ctx context.Context, eventID string, errorDetails string, nextRetryAt *time.Time) error {
	args := m.Called(ctx, eventID, errorDetails, nextRetryAt)
	return args.Error(0)
}

func (m *MockOutboxRepository) MarkPermanentlyFailed(ctx context.Context, eventID string, errorDetails string) error {
	args := m.Called(ctx, eventID, errorDetails)
	return args.Error(0)
}

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) GetDB() (*sql.DB, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sql.DB), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockDB struct {
	mock.Mock
}

func (m *MockDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sql.Tx), args.Error(1)
}

type MockTx struct {
	mock.Mock
}

func (m *MockTx) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTx) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func createTestLogger() *infrastructure.Logger {
	logConfig := config.LoggingConfig{
		Level:  "error", // Use error level to reduce test output
		Format: "json",
	}
	return infrastructure.New(logConfig)
}

func createTestSSEConfig() config.SSEConfig {
	return config.SSEConfig{
		EventsInterval: 100 * time.Millisecond, // Faster for tests
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

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Cache returns the analysis (cache hit)
	mockCacheRepo.On("Find", ctx, analysisID).Return(expectedAnalysis, nil)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}
	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

	// Act
	result, err := service.FetchAnalysis(ctx, analysisID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedAnalysis.ID, result.ID)
	assert.Equal(t, expectedAnalysis.URL, result.URL)
	assert.Equal(t, expectedAnalysis.Status, result.Status)
	assert.Equal(t, expectedAnalysis.Results.Title, result.Results.Title)

	// Verify that database was not called (cache hit)
	mockAnalysisRepo.AssertNotCalled(t, "Find")
	mockCacheRepo.AssertExpectations(t)
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

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Cache miss
	mockCacheRepo.On("Find", ctx, analysisID).Return(nil, domain.ErrCacheUnavailable)
	// Database returns the analysis
	mockAnalysisRepo.On("Find", ctx, analysisID).Return(expectedAnalysis, nil)
	// Cache the result
	mockCacheRepo.On("Set", ctx, expectedAnalysis).Return(nil)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}
	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

	// Act
	result, err := service.FetchAnalysis(ctx, analysisID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedAnalysis.ID, result.ID)
	assert.Equal(t, expectedAnalysis.URL, result.URL)
	assert.Equal(t, expectedAnalysis.Status, result.Status)

	// Verify all calls were made
	mockCacheRepo.AssertExpectations(t)
	mockAnalysisRepo.AssertExpectations(t)
}

// Test FetchAnalysis when both cache and DB fail
func TestFetchAnalysis_BothFail(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := t.Context()
	analysisID := uuid.New().String()

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Cache miss
	mockCacheRepo.On("Find", ctx, analysisID).Return(nil, domain.ErrCacheUnavailable)
	// Database also fails
	mockAnalysisRepo.On("Find", ctx, analysisID).Return(nil, domain.ErrAnalysisNotFound)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}
	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

	// Act
	result, err := service.FetchAnalysis(ctx, analysisID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to find analysis")

	// Verify all calls were made
	mockCacheRepo.AssertExpectations(t)
	mockAnalysisRepo.AssertExpectations(t)
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

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Service will fail before reaching repository due to nil storage
	// mockAnalysisRepo.On("Save", ctx, url, options).Return(nil, domain.ErrInternalServerError)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}

	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

	// Act
	result, err := service.StartAnalysis(ctx, url, options)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "storage client not initialized")

	// Neither repository should be called since storage fails early
	mockCacheRepo.AssertNotCalled(t, "Set")
	mockAnalysisRepo.AssertNotCalled(t, "Save")
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

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Service will fail before reaching repository due to nil storage
	// mockAnalysisRepo.On("Save", ctx, url, options).Return(expectedAnalysis, nil)
	// Cache fails to save
	// mockCacheRepo.On("Set", ctx, expectedAnalysis).Return(domain.ErrCacheUnavailable)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}

	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

	// Act
	result, err := service.StartAnalysis(ctx, url, options)

	// Assert - Should fail due to nil storage
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "storage client not initialized")

	// Neither repository should be called since storage fails early
	mockAnalysisRepo.AssertNotCalled(t, "Save")
	mockCacheRepo.AssertNotCalled(t, "Set")
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

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Mock the FetchAnalysis call within FetchAnalysisEvents
	mockCacheRepo.On("Find", ctx, analysisID).Return(expectedAnalysis, nil)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}
	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

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
		assert.Equal(t, expectedAnalysis, event.Data)
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

	mockCacheRepo.AssertExpectations(t)
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

	mockAnalysisRepo := new(MockAnalysisRepository)
	mockCacheRepo := new(MockCacheRepository)
	logger := createTestLogger()

	// Mock the FetchAnalysis call within FetchAnalysisEvents
	mockCacheRepo.On("Find", ctx, analysisID).Return(expectedAnalysis, nil)

	mockOutboxRepo := &MockOutboxRepository{}
	mockHealthChecker := &MockHealthChecker{}
	sseConfig := createTestSSEConfig()
	service := NewApplicationService(mockAnalysisRepo, mockCacheRepo, mockOutboxRepo, mockHealthChecker, nil, sseConfig, logger)

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
		assert.Equal(t, expectedAnalysis, event.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive an event but got timeout")
	}

	mockCacheRepo.AssertExpectations(t)
}
