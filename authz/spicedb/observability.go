package spicedb

import (
	"context"
	"strings"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	metricSpiceDBRequestsTotal  = "kernel_authz_spicedb_requests_total"
	metricSpiceDBRequestSeconds = "kernel_authz_spicedb_request_duration_seconds"
)

func spiceDBLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("authz.spicedb").With(logx.String("backend", "spicedb"))
}

func registerSpiceDBMetrics(cfg Config) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	cfg.Metrics.NewCounter(metricSpiceDBRequestsTotal, "Total SpiceDB backend gRPC calls")
	cfg.Metrics.NewHistogram(metricSpiceDBRequestSeconds, "SpiceDB backend gRPC call latency in seconds", 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
}

func observabilityDialOptions(cfg Config, logger logx.Logger) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(spiceDBUnaryClientInterceptor(cfg, logger)),
		grpc.WithChainStreamInterceptor(spiceDBStreamClientInterceptor(cfg, logger)),
	}
}

func spiceDBUnaryClientInterceptor(cfg Config, logger logx.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		observeSpiceDBCall(cfg, logger, ctx, method, false, start, err)
		return err
	}
}

func spiceDBStreamClientInterceptor(cfg Config, logger logx.Logger) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		stream, err := streamer(ctx, desc, cc, method, opts...)
		observeSpiceDBCall(cfg, logger, ctx, method, true, start, err)
		return stream, err
	}
}

func observeSpiceDBCall(cfg Config, logger logx.Logger, ctx context.Context, method string, stream bool, start time.Time, err error) {
	elapsed := time.Since(start)
	service, operation := splitMethod(method)
	grpcCode := "OK"
	code := errorx.CodeOK.String()
	statusClass := "2xx"
	if err != nil {
		grpcCode = status.Code(err).String()
		code = errorx.CodeUnavailable.String()
		statusClass = "5xx"
	}
	if cfg.MetricsEnabled && cfg.Metrics != nil {
		labels := []string{"service", service, "method", operation, "grpc_code", grpcCode, "code", code}
		cfg.Metrics.IncrementCounter(ctx, metricSpiceDBRequestsTotal, labels...)
		cfg.Metrics.RecordHistogram(ctx, metricSpiceDBRequestSeconds, elapsed.Seconds(), labels...)
	}
	fields := []logx.Field{
		logx.String("service", service),
		logx.String("grpc_method", operation),
		logx.String("operation", method),
		logx.String("grpc_code", grpcCode),
		logx.String("status_class", statusClass),
		logx.Bool("stream", stream),
		logx.Duration("elapsed", elapsed),
	}
	if err != nil {
		logger.Error("spicedb backend call failed", append(fields, logx.Err(errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("spicedb backend call failed"))))...)
		return
	}
	logger.Debug("spicedb backend call", fields...)
}

func splitMethod(method string) (service string, operation string) {
	method = strings.TrimPrefix(method, "/")
	parts := strings.Split(method, "/")
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}
