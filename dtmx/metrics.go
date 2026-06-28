package dtmx

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/metricsx"
)

const (
	metricOperations = "kernel_dtmx_operations_total"
	metricDuration   = "kernel_dtmx_operation_duration_seconds"
)

func registerMetrics(m metricsx.Manager) {
	if m == nil {
		return
	}
	m.NewCounter(metricOperations, "DTM operations total")
	m.NewHistogram(metricDuration, "DTM operation duration seconds", metricsx.DefaultBuckets...)
}

func recordOperation(ctx context.Context, cfg Config, operation, status, code string, elapsed time.Duration) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	cfg.Metrics.IncrementCounter(ctx, metricOperations, "operation", operation, "status", status, "code", code)
	cfg.Metrics.RecordHistogram(ctx, metricDuration, elapsed.Seconds(), "operation", operation, "status", status, "code", code)
}
