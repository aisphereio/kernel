package dtmx

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

const (
	metricOperationsTotal   = "kernel_dtmx_operations_total"
	metricOperationDuration = "kernel_dtmx_operation_duration_seconds"
)

func componentLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("dtmx").With(
		logx.String("driver", cfg.Driver),
		logx.String("protocol", cfg.Protocol),
	)
}

func registerMetrics(cfg Config) {
	if !cfg.MetricsEnabled {
		return
	}
	m := metricsx.Ensure(cfg.Metrics)
	m.NewCounter(metricOperationsTotal, "Total distributed transaction operations")
	m.NewHistogram(metricOperationDuration, "Distributed transaction operation duration", metricsx.DefaultBuckets...)
}

func recordOperation(ctx context.Context, cfg Config, operation, status, code string, elapsed time.Duration) {
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
