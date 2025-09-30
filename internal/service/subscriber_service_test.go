package service

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/mocks"
)

type (
	SubscriberServiceTestSuite struct {
		suite.Suite
		mocks   *mockDependencies
		service SubscriberService
	}

	mockDependencies struct {
		analysisRepo *mocks.FakeAnalysisRepository
		outboxRepo   *mocks.FakeOutboxRepository
		cacheRepo    *mocks.FakeCacheRepository
		webFetcher   *mocks.FakeWebFetcher
		htmlAnalyzer *mocks.FakeHTMLAnalyzer
		linkChecker  *mocks.FakeLinkChecker
		metrics      *mocks.FakeMetrics
		logger       infrastructure.Logger
	}
)

func TestSubscriberServiceTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SubscriberServiceTestSuite))
}

func (s *SubscriberServiceTestSuite) SetupTest() {
	s.mocks = &mockDependencies{
		analysisRepo: &mocks.FakeAnalysisRepository{},
		outboxRepo:   &mocks.FakeOutboxRepository{},
		cacheRepo:    &mocks.FakeCacheRepository{},
		webFetcher:   &mocks.FakeWebFetcher{},
		htmlAnalyzer: &mocks.FakeHTMLAnalyzer{},
		linkChecker:  &mocks.FakeLinkChecker{},
		metrics:      &mocks.FakeMetrics{},
		logger:       infrastructure.NewTestLogger(),
	}

	s.service = NewSubscriberService(
		s.mocks.analysisRepo,
		s.mocks.outboxRepo,
		s.mocks.cacheRepo,
		s.mocks.webFetcher,
		s.mocks.htmlAnalyzer,
		s.mocks.linkChecker,
		s.mocks.logger,
		s.mocks.metrics,
	)
}

func (s *SubscriberServiceTestSuite) TestProcessAnalysisRequest_InvalidatesCache_OnStatusUpdate() {
	t := s.T()

	analysisID := uuid.New()
	url := "https://example.com"
	payload := s.createTestPayload(analysisID, url)
	outboxEvent := s.createTestOutboxEvent(analysisID)
	webContent := s.createTestWebContent(url)
	analysisData := s.createTestAnalysisData()
	analysis := &domain.Analysis{
		ID:     analysisID,
		URL:    url,
		Status: domain.StatusCompleted,
	}

	s.setupSuccessfulAnalysisFlow(outboxEvent, webContent, analysisData, analysis)

	result, err := s.service.ProcessAnalysisRequest(t.Context(), payload)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().True(result.Success)
	s.Require().GreaterOrEqual(s.mocks.cacheRepo.DeleteCallCount(), 1,
		"Cache should be invalidated after status update to in_progress")

	if s.mocks.cacheRepo.DeleteCallCount() > 0 {
		_, firstDeleteID := s.mocks.cacheRepo.DeleteArgsForCall(0)
		s.Require().Equal(analysisID.String(), firstDeleteID,
			"First cache delete should be for the analysis that was updated to in_progress")
	}
}

func (s *SubscriberServiceTestSuite) TestProcessAnalysisRequest_InvalidatesCache_OnFetchError() {
	t := s.T()

	analysisID := uuid.New()
	url := "https://example.com"
	payload := domain.AnalysisRequestPayload{
		AnalysisID: analysisID,
		URL:        url,
		Options: domain.AnalysisOptions{
			Timeout: 30 * time.Second,
		},
		Priority:  domain.PriorityNormal,
		CreatedAt: time.Now(),
	}
	outboxEvent := s.createTestOutboxEvent(analysisID)
	fetchError := errors.New("failed to fetch URL")

	s.setupFailedFetchFlow(outboxEvent, fetchError)

	result, err := s.service.ProcessAnalysisRequest(t.Context(), payload)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().False(result.Success)
	s.Require().Equal("FETCH_ERROR", result.ErrorCode)
	s.Require().Equal(1, s.mocks.analysisRepo.MarkFailedCallCount(),
		"MarkFailed should be called once")
	s.Require().GreaterOrEqual(s.mocks.cacheRepo.DeleteCallCount(), 1,
		"Cache should be invalidated after marking analysis as failed")

	if s.mocks.cacheRepo.DeleteCallCount() > 0 {
		found := false
		for index := 0; index < s.mocks.cacheRepo.DeleteCallCount(); index++ {
			_, deleteID := s.mocks.cacheRepo.DeleteArgsForCall(index)
			if deleteID == analysisID.String() {
				found = true

				break
			}
		}
		s.Require().True(found, "Cache delete should be called for the failed analysis")
	}
}

func (s *SubscriberServiceTestSuite) TestProcessAnalysisRequest_ContinuesOnCacheDeleteError() {
	t := s.T()

	analysisID := uuid.New()
	url := "https://example.com"
	payload := s.createTestPayload(analysisID, url)
	outboxEvent := s.createTestOutboxEvent(analysisID)
	webContent := s.createTestWebContent(url)
	analysisData := s.createTestAnalysisData()
	analysis := &domain.Analysis{
		ID:     analysisID,
		URL:    url,
		Status: domain.StatusCompleted,
	}
	cacheError := errors.New("cache unavailable")

	s.setupSuccessfulAnalysisFlow(outboxEvent, webContent, analysisData, analysis)
	s.mocks.cacheRepo.DeleteReturns(cacheError)

	result, err := s.service.ProcessAnalysisRequest(t.Context(), payload)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().True(result.Success)
	s.Require().GreaterOrEqual(s.mocks.cacheRepo.DeleteCallCount(), 1,
		"Cache delete should be attempted even if it fails")
	s.Require().Equal(1, s.mocks.outboxRepo.MarkCompletedCallCount(),
		"Outbox event should be marked as completed despite cache error")
}

func (s *SubscriberServiceTestSuite) TestProcessAnalysisRequest_NilCacheRepo() {
	t := s.T()

	analysisID := uuid.New()
	url := "https://example.com"
	payload := domain.AnalysisRequestPayload{
		AnalysisID: analysisID,
		URL:        url,
		Options: domain.AnalysisOptions{
			Timeout: 30 * time.Second,
		},
		Priority:  domain.PriorityNormal,
		CreatedAt: time.Now(),
	}
	outboxEvent := s.createTestOutboxEvent(analysisID)
	fetchError := errors.New("failed to fetch URL")

	serviceWithoutCache := NewSubscriberService(
		s.mocks.analysisRepo,
		s.mocks.outboxRepo,
		nil,
		s.mocks.webFetcher,
		s.mocks.htmlAnalyzer,
		s.mocks.linkChecker,
		s.mocks.logger,
		s.mocks.metrics,
	)

	s.setupFailedFetchFlow(outboxEvent, fetchError)

	result, err := serviceWithoutCache.ProcessAnalysisRequest(t.Context(), payload)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().False(result.Success)
	s.Require().Equal(1, s.mocks.analysisRepo.MarkFailedCallCount(),
		"MarkFailed should be called even with nil cache repo")
}

func (s *SubscriberServiceTestSuite) createTestPayload(analysisID uuid.UUID, url string) domain.AnalysisRequestPayload {
	return domain.AnalysisRequestPayload{
		AnalysisID: analysisID,
		URL:        url,
		Options: domain.AnalysisOptions{
			IncludeHeadings: true,
			CheckLinks:      false,
			DetectForms:     false,
			Timeout:         30 * time.Second,
		},
		Priority:  domain.PriorityNormal,
		CreatedAt: time.Now(),
	}
}

func (s *SubscriberServiceTestSuite) createTestOutboxEvent(analysisID uuid.UUID) *domain.OutboxEvent {
	return &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateID:   analysisID,
		AggregateType: "analysis",
		EventType:     domain.OutboxEventAnalysisRequested,
		Priority:      domain.PriorityNormal,
		Status:        domain.OutboxStatusPending,
		CreatedAt:     time.Now(),
	}
}

func (s *SubscriberServiceTestSuite) createTestWebContent(url string) *domain.WebPageContent {
	return &domain.WebPageContent{
		URL:           url,
		HTML:          "<html><head><title>Test</title></head><body><h1>Test</h1></body></html>",
		FetchDuration: 100 * time.Millisecond,
	}
}

func (s *SubscriberServiceTestSuite) createTestAnalysisData() *domain.AnalysisData {
	return &domain.AnalysisData{
		Title:       "Test",
		HTMLVersion: domain.HTML5,
		HeadingCounts: domain.HeadingCounts{
			H1: 1,
		},
	}
}

func (s *SubscriberServiceTestSuite) setupSuccessfulAnalysisFlow(
	outboxEvent *domain.OutboxEvent,
	webContent *domain.WebPageContent,
	analysisData *domain.AnalysisData,
	analysis *domain.Analysis,
) {
	s.mocks.outboxRepo.GetByAggregateIDReturns(outboxEvent, nil)
	s.mocks.outboxRepo.MarkProcessedReturns(nil)
	s.mocks.analysisRepo.UpdateStatusReturns(nil)
	s.mocks.webFetcher.FetchReturns(webContent, nil)
	s.mocks.htmlAnalyzer.AnalyzeReturns(analysisData, nil)
	s.mocks.analysisRepo.FindByContentHashReturns(nil, sql.ErrNoRows)
	s.mocks.analysisRepo.UpdateReturns(nil)
	s.mocks.analysisRepo.FindReturns(analysis, nil)
	s.mocks.analysisRepo.UpdateCompletionDurationReturns(nil)
	s.mocks.outboxRepo.MarkCompletedReturns(nil)
	s.mocks.cacheRepo.DeleteReturns(nil)
	s.mocks.cacheRepo.SetReturns(nil)
}

func (s *SubscriberServiceTestSuite) setupFailedFetchFlow(outboxEvent *domain.OutboxEvent, fetchError error) {
	s.mocks.outboxRepo.GetByAggregateIDReturns(outboxEvent, nil)
	s.mocks.outboxRepo.MarkProcessedReturns(nil)
	s.mocks.analysisRepo.UpdateStatusReturns(nil)
	s.mocks.webFetcher.FetchReturns(nil, fetchError)
	s.mocks.analysisRepo.MarkFailedReturns(nil)
	s.mocks.cacheRepo.DeleteReturns(nil)
}
