package dtm

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/dtmx"
	"github.com/aisphereio/kernel/metricsx"
)

const (
	metricOperationsTotal   = "kernel_dtmx_operations_total"
	metricOperationDuration = "kernel_dtmx_operation_duration_seconds"
)

func record(ctx context.Context, cfg dtmx.Config, operation, status, code string, elapsed time.Duration) {
	if !cfg.MetricsEnabled {
		return
	}
	m := metricsx.Ensure(cfg.Metrics)
	m.IncrementCounter(ctx, metricOperationsTotal,
		"driver", cfg.Driver,
		"protocol", cfg.Protocol,
		"operation", operation,
		"status", status,
		"code", code,
	)
	m.RecordHistogram(ctx, metricOperationDuration, elapsed.Seconds(),
		"driver", cfg.Driver,
		"protocol", cfg.Protocol,
		"operation", operation,
		"status", status,
		"code", code,
	)
}
