package adapters

import (
	"context"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
)

type MetricsAdapter struct {
	metrics infrastructure.Metrics
}

func NewMetricsAdapter(metrics infrastructure.Metrics) decorator.MetricsClient {
	return &MetricsAdapter{
		metrics: metrics,
	}
}

func (m *MetricsAdapter) Inc(key string, value int) {
	ctx := context.Background()
	duration := time.Duration(value) * time.Second

	m.metrics.RecordAnalysisRequest(ctx, duration, true, "")
}
