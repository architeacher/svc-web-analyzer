package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/mocks"
)

type (
	ApplicationServiceTestSuite struct {
		suite.Suite
		fakeAnalysisRepo  *mocks.FakeAnalysisRepository
		fakeCacheRepo     *mocks.FakeCacheRepository
		fakeOutboxRepo    *mocks.FakeOutboxRepository
		fakeHealthChecker *mocks.FakeHealthChecker
		logger            infrastructure.Logger
		sseConfig         config.SSEConfig
		outboxConfig      config.OutboxConfig
		service           ApplicationService
	}
)

func TestApplicationServiceTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ApplicationServiceTestSuite))
}

func (s *ApplicationServiceTestSuite) SetupTest() {
	s.fakeAnalysisRepo = &mocks.FakeAnalysisRepository{}
	s.fakeCacheRepo = &mocks.FakeCacheRepository{}
	s.fakeOutboxRepo = &mocks.FakeOutboxRepository{}
	s.fakeHealthChecker = &mocks.FakeHealthChecker{}
	s.logger = infrastructure.NewTestLogger()
	s.sseConfig = s.createSSEConfig()
	s.outboxConfig = s.createOutboxConfig()
	s.service = NewApplicationService(
		s.fakeAnalysisRepo,
		s.fakeOutboxRepo,
		s.fakeCacheRepo,
		s.fakeHealthChecker,
		nil,
		s.sseConfig,
		s.outboxConfig,
		s.logger,
	)
}

func (s *ApplicationServiceTestSuite) TestFetchAnalysis_CacheHit() {
	expectedAnalysis := s.createAnalysis(domain.StatusCompleted)
	s.fakeCacheRepo.FindReturns(expectedAnalysis, nil)

	result, err := s.service.FetchAnalysis(s.T().Context(), expectedAnalysis.ID.String())

	s.Require().NoError(err)
	s.Require().Equal(expectedAnalysis.ID, result.ID)
	s.Require().Equal(expectedAnalysis.URL, result.URL)
	s.Require().Equal(expectedAnalysis.Status, result.Status)
	s.Require().Equal(expectedAnalysis.Results.Title, result.Results.Title)
	s.Require().Equal(0, s.fakeAnalysisRepo.FindCallCount())
	s.Require().Equal(1, s.fakeCacheRepo.FindCallCount())
}

func (s *ApplicationServiceTestSuite) TestFetchAnalysis_CacheMiss() {
	expectedAnalysis := s.createAnalysis(domain.StatusCompleted)
	s.fakeCacheRepo.FindReturns(nil, domain.ErrCacheUnavailable)
	s.fakeAnalysisRepo.FindReturns(expectedAnalysis, nil)
	s.fakeCacheRepo.SetReturns(nil)

	result, err := s.service.FetchAnalysis(s.T().Context(), expectedAnalysis.ID.String())

	s.Require().NoError(err)
	s.Require().Equal(expectedAnalysis.ID, result.ID)
	s.Require().Equal(expectedAnalysis.URL, result.URL)
	s.Require().Equal(expectedAnalysis.Status, result.Status)
	s.Require().Equal(1, s.fakeCacheRepo.FindCallCount())
	s.Require().Equal(1, s.fakeAnalysisRepo.FindCallCount())
	s.Require().Equal(1, s.fakeCacheRepo.SetCallCount())
}

func (s *ApplicationServiceTestSuite) TestFetchAnalysis_BothFail() {
	analysisID := uuid.New().String()
	s.fakeCacheRepo.FindReturns(nil, domain.ErrCacheUnavailable)
	s.fakeAnalysisRepo.FindReturns(nil, domain.ErrAnalysisNotFound)

	result, err := s.service.FetchAnalysis(s.T().Context(), analysisID)

	s.Require().Error(err)
	s.Require().Nil(result)
	s.Require().Contains(err.Error(), "failed to find analysis")
	s.Require().Equal(1, s.fakeCacheRepo.FindCallCount())
	s.Require().Equal(1, s.fakeAnalysisRepo.FindCallCount())
}

func (s *ApplicationServiceTestSuite) TestStartAnalysis_Success() {
	if testing.Short() {
		s.T().Skip("skipping test that requires database transactions")
	}

	s.T().Skip("This should be converted to a proper integration test using testcontainers")
}

func (s *ApplicationServiceTestSuite) TestStartAnalysis_DBFails() {
	if testing.Short() {
		s.T().Skip("skipping test that requires database transactions")
	}

	s.T().Skip("This should be converted to a proper integration test using testcontainers")
}

func (s *ApplicationServiceTestSuite) TestStartAnalysis_CacheFailsDBSucceeds() {
	if testing.Short() {
		s.T().Skip("skipping test that requires database transactions")
	}

	s.T().Skip("This should be converted to a proper integration test using testcontainers")
}

func (s *ApplicationServiceTestSuite) TestFetchAnalysisEvents_CompletedAnalysis() {
	expectedAnalysis := s.createAnalysis(domain.StatusCompleted)
	s.fakeCacheRepo.FindReturns(expectedAnalysis, nil)

	eventsChan, err := s.service.FetchAnalysisEvents(s.T().Context(), expectedAnalysis.ID.String())

	s.Require().NoError(err)
	s.Require().NotNil(eventsChan)

	event := s.readEventWithTimeout(eventsChan)
	s.Require().Equal(domain.EventTypeCompleted, event.Type)
	s.Require().Equal(expectedAnalysis.ID.String(), event.EventID)
	s.Require().Equal(expectedAnalysis, event.Payload)

	s.assertChannelClosed(eventsChan)
}

func (s *ApplicationServiceTestSuite) TestFetchAnalysisEvents_FailedAnalysis() {
	expectedAnalysis := s.createFailedAnalysis()
	s.fakeCacheRepo.FindReturns(expectedAnalysis, nil)

	eventsChan, err := s.service.FetchAnalysisEvents(s.T().Context(), expectedAnalysis.ID.String())

	s.Require().NoError(err)
	s.Require().NotNil(eventsChan)

	event := s.readEventWithTimeout(eventsChan)
	s.Require().Equal(domain.EventTypeFailed, event.Type)
	s.Require().Equal(expectedAnalysis.ID.String(), event.EventID)
	s.Require().Equal(expectedAnalysis, event.Payload)
}

func (s *ApplicationServiceTestSuite) createSSEConfig() config.SSEConfig {
	return config.SSEConfig{
		EventsInterval:    100 * time.Millisecond,
		HeartbeatInterval: 100 * time.Millisecond,
	}
}

func (s *ApplicationServiceTestSuite) createOutboxConfig() config.OutboxConfig {
	return config.OutboxConfig{
		MaxRetries: config.MaxRetriesByPriority{
			Low:    3,
			Normal: 5,
			High:   7,
			Urgent: 10,
		},
	}
}

func (s *ApplicationServiceTestSuite) createAnalysis(status domain.AnalysisStatus) *domain.Analysis {
	return &domain.Analysis{
		ID:        uuid.New(),
		URL:       "https://example.com",
		Status:    status,
		CreatedAt: time.Now(),
		Results: &domain.AnalysisData{
			Title: "Example Title",
		},
	}
}

func (s *ApplicationServiceTestSuite) createFailedAnalysis() *domain.Analysis {
	return &domain.Analysis{
		ID:        uuid.New(),
		URL:       "https://example.com",
		Status:    domain.StatusFailed,
		CreatedAt: time.Now(),
		Error: &domain.AnalysisError{
			Code:    "FETCH_ERROR",
			Message: "Failed to fetch URL",
		},
	}
}

func (s *ApplicationServiceTestSuite) createAnalysisOptions() domain.AnalysisOptions {
	return domain.AnalysisOptions{
		IncludeHeadings: true,
		CheckLinks:      true,
		DetectForms:     true,
		Timeout:         30 * time.Second,
	}
}

func (s *ApplicationServiceTestSuite) readEventWithTimeout(eventsChan <-chan domain.AnalysisEvent) domain.AnalysisEvent {
	select {
	case event := <-eventsChan:
		return event
	case <-time.After(1 * time.Second):
		s.T().Fatal("Expected to receive an event but got timeout")

		return domain.AnalysisEvent{}
	}
}

func (s *ApplicationServiceTestSuite) assertChannelClosed(eventsChan <-chan domain.AnalysisEvent) {
	select {
	case _, ok := <-eventsChan:
		require.False(s.T(), ok, "Channel should be closed")
	case <-time.After(100 * time.Millisecond):
		s.T().Fatal("Channel should be closed")
	}
}
