package dbx

import (
	"context"
	"errors"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

const (
	CodeNoRows              = errorx.Code("DBX_NO_ROWS")
	CodeDuplicateKey        = errorx.Code("DBX_DUPLICATE_KEY")
	CodeTimeout             = errorx.Code("DBX_TIMEOUT")
	CodeSchemaNotReady      = errorx.Code("DBX_SCHEMA_NOT_READY")
	CodeDatabaseNotExist    = errorx.Code("DBX_DATABASE_NOT_EXIST")
	CodeForeignKeyViolation = errorx.Code("DBX_FOREIGN_KEY_VIOLATION")
	CodeClosed              = errorx.Code("DBX_CLOSED")
	CodeInvalidConfig       = errorx.Code("DBX_INVALID_CONFIG")
	CodeUnknownDriver       = errorx.Code("DBX_UNKNOWN_DRIVER")
	CodeTxStateInvalid      = errorx.Code("DBX_TX_STATE_INVALID")
	CodeUnsafeUpsert        = errorx.Code("DBX_UNSAFE_UPSERT")
	CodeNoEffect            = errorx.Code("DBX_NO_EFFECT")
	CodeOperationFailed     = errorx.Code("DBX_OPERATION_FAILED")
)

const (
	metricDBXOperationsTotal  = "kernel_dbx_operations_total"
	metricDBXOperationSeconds = "kernel_dbx_operation_duration_seconds"
)

func dbxLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("dbx").With(logx.String("driver", cfg.Driver))
}

func registerDBXMetrics(cfg Config) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	cfg.Metrics.NewCounter(metricDBXOperationsTotal, "Total dbx operations")
	cfg.Metrics.NewHistogram(metricDBXOperationSeconds, "dbx operation latency in seconds", 0.001, 0.003, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5)
}

// NormalizeError converts dbx sentinel/driver errors into Kernel errorx values
// while preserving the original error chain for errors.Is/errors.As callers.
func NormalizeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errorx.As(err); ok {
		return err
	}
	code, status, message, retryable := classifyError(err)
	return errorx.Wrap(err, code,
		errorx.WithMessage(message),
		errorx.WithHTTPStatus(status),
		errorx.WithRetryable(retryable),
		errorx.WithMetadata("component", "dbx"),
	)
}

func classifyError(err error) (errorx.Code, int, string, bool) {
	switch {
	case errors.Is(err, ErrNoRows):
		return CodeNoRows, errorx.HTTPStatusNotFound, "database row not found", false
	case errors.Is(err, ErrDuplicateKey):
		return CodeDuplicateKey, errorx.HTTPStatusConflict, "database duplicate key", false
	case errors.Is(err, ErrTimeout):
		return CodeTimeout, errorx.HTTPStatusGatewayTimeout, "database operation timed out", true
	case errors.Is(err, context.Canceled):
		return CodeTimeout, errorx.HTTPStatusClientClosedRequest, "database operation canceled", false
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout, errorx.HTTPStatusGatewayTimeout, "database operation timed out", true
	case errors.Is(err, ErrSchemaNotReady):
		return CodeSchemaNotReady, errorx.HTTPStatusServiceUnavailable, "database schema is not ready", true
	case errors.Is(err, ErrDatabaseNotExist):
		return CodeDatabaseNotExist, errorx.HTTPStatusServiceUnavailable, "database does not exist", true
	case errors.Is(err, ErrForeignKeyViolation):
		return CodeForeignKeyViolation, errorx.HTTPStatusConflict, "database foreign key violation", false
	case errors.Is(err, ErrClosed):
		return CodeClosed, errorx.HTTPStatusServiceUnavailable, "database is closed", true
	case errors.Is(err, ErrNilConfig):
		return CodeInvalidConfig, errorx.HTTPStatusBadRequest, "database config is invalid", false
	case errors.Is(err, ErrUnknownDriver):
		return CodeUnknownDriver, errorx.HTTPStatusBadRequest, "database driver is not registered", false
	case errors.Is(err, ErrTxRolledBack), errors.Is(err, ErrTxCommitted):
		return CodeTxStateInvalid, errorx.HTTPStatusConflict, "database transaction state is invalid", false
	case errors.Is(err, ErrUnscopedRequired), errors.Is(err, ErrUnsafeUpsert):
		return CodeUnsafeUpsert, errorx.HTTPStatusBadRequest, "unsafe database operation blocked", false
	case errors.Is(err, ErrNoEffect):
		return CodeNoEffect, errorx.HTTPStatusNotFound, "database operation affected no rows", false
	default:
		return CodeOperationFailed, errorx.HTTPStatusInternalServerError, "database operation failed", false
	}
}

func observeDBInit(cfg Config, started time.Time, err error) error {
	logger := dbxLogger(cfg)
	elapsed := time.Since(started)
	if err == nil {
		logger.Info("dbx opened", logx.Duration("elapsed", elapsed), logx.Bool("metrics_enabled", cfg.MetricsEnabled), logx.Bool("debug", cfg.Debug))
		return nil
	}
	nerr := NormalizeError(err)
	logger.Error("dbx open failed", logx.Duration("elapsed", elapsed), logx.Err(nerr))
	return nerr
}

func observeDBOperation(cfg Config, ctx context.Context, operation string, started time.Time, err error) error {
	elapsed := time.Since(started)
	nerr := NormalizeError(err)
	status := "ok"
	code := errorx.CodeOK.String()
	if nerr != nil {
		status = "error"
		code = errorx.CodeOf(nerr).String()
	}
	if cfg.MetricsEnabled && cfg.Metrics != nil {
		labels := []string{"driver", cfg.Driver, "operation", operation, "status", status, "code", code}
		cfg.Metrics.IncrementCounter(ctx, metricDBXOperationsTotal, labels...)
		cfg.Metrics.RecordHistogram(ctx, metricDBXOperationSeconds, elapsed.Seconds(), labels...)
	}
	if nerr != nil && !errors.Is(nerr, ErrNoRows) && !errors.Is(nerr, ErrNoEffect) {
		dbxLogger(cfg).Error("dbx operation failed", logx.String("operation", operation), logx.Duration("elapsed", elapsed), logx.Err(nerr))
	}
	return nerr
}

func observeDBOperationRaw(cfg Config, ctx context.Context, operation string, started time.Time, err error) error {
	elapsed := time.Since(started)
	status := "ok"
	code := errorx.CodeOK.String()
	if err != nil {
		status = "error"
		code = errorx.CodeOf(err).String()
	}
	if cfg.MetricsEnabled && cfg.Metrics != nil {
		labels := []string{"driver", cfg.Driver, "operation", operation, "status", status, "code", code}
		cfg.Metrics.IncrementCounter(ctx, metricDBXOperationsTotal, labels...)
		cfg.Metrics.RecordHistogram(ctx, metricDBXOperationSeconds, elapsed.Seconds(), labels...)
	}
	if err != nil {
		dbxLogger(cfg).Error("dbx operation failed", logx.String("operation", operation), logx.Duration("elapsed", elapsed), logx.Err(err))
	}
	return err
}
