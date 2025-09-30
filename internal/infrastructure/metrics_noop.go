package infrastructure

import (
	"context"
	"net/http"
	"time"
)

type (
	NoOp struct{}

	NoOpMetrics struct{}
)

func (d NoOp) Inc(_ string, _ int) {
}

func (n *NoOpMetrics) RecordHTTPRequest(_ context.Context, _, _ string, _ int, _ time.Duration, _, _ int64) {
}

func (n *NoOpMetrics) RecordAnalysisRequest(_ context.Context, _ time.Duration, _ bool, _ string) {
}

func (n *NoOpMetrics) RecordOutboxEvent(_ context.Context, _ bool, _ string) {
}

func (n *NoOpMetrics) RecordLinkCheck(_ context.Context, _ bool, _ string) {
}

func (n *NoOpMetrics) RecordFetchTime(_ context.Context, _ time.Duration) {
}

func (n *NoOpMetrics) RecordProcessingTime(_ context.Context, _ time.Duration) {
}

func (n *NoOpMetrics) Handler() http.Handler {
	return http.NotFoundHandler()
}

func (n *NoOpMetrics) Shutdown(_ context.Context) error {
	return nil
}
