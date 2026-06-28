package grpc

import (
	"context"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

const (
	metricGRPCServerRequestsTotal  = "kernel_grpc_server_requests_total"
	metricGRPCServerRequestSeconds = "kernel_grpc_server_request_duration_seconds"
	metricGRPCServerInflight       = "kernel_grpc_server_inflight_requests"
)

func (s *Server) registerMetrics() {
	if s.metrics == nil {
		return
	}
	s.metrics.NewCounter(metricGRPCServerRequestsTotal, "Total gRPC server requests")
	s.metrics.NewHistogram(metricGRPCServerRequestSeconds, "gRPC server request latency in seconds", 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
	s.metrics.NewUpDownCounter(metricGRPCServerInflight, "Current in-flight gRPC server requests")
}

func (s *Server) injectLogContext(ctx context.Context, md metadata.MD) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	fields := make([]logx.Field, 0, 3)
	if requestID := firstMetadata(md, "x-request-id", "request-id"); requestID != "" {
		fields = append(fields, logx.String("request_id", requestID))
	}
	if traceID := firstMetadata(md, "x-trace-id", "trace-id"); traceID != "" {
		fields = append(fields, logx.String("trace_id", traceID))
	} else if traceID := traceIDFromMetadata(md); traceID != "" {
		fields = append(fields, logx.String("trace_id", traceID))
	}
	if len(fields) == 0 {
		return logx.Inject(ctx, s.logger)
	}
	return logx.Inject(ctx, s.logger, fields...)
}

func firstMetadata(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		values := md.Get(key)
		if len(values) > 0 && values[0] != "" {
			return values[0]
		}
	}
	return ""
}

func traceIDFromMetadata(md metadata.MD) string {
	traceparent := firstMetadata(md, "traceparent")
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) < 4 || len(parts[1]) != 32 {
		return ""
	}
	return parts[1]
}

func (s *Server) beforeRPC(ctx context.Context, operation string) time.Time {
	if s.metricsEnabled && s.metrics != nil {
		service, method := splitFullMethod(operation)
		s.metrics.DeltaUpDownCounter(ctx, metricGRPCServerInflight, 1,
			"service", service,
			"method", method,
			"operation", operation,
		)
	}
	return time.Now()
}

func (s *Server) afterRPC(ctx context.Context, operation string, stream bool, start time.Time, err error) {
	latency := time.Since(start)
	service, method := splitFullMethod(operation)
	status := errorx.HTTPStatusOf(err)
	code := errorx.CodeOf(err).String()
	grpcCode := errorx.GRPCCodeOf(err)
	statusClass := strconv.Itoa(status/100) + "xx"

	if s.metricsEnabled && s.metrics != nil {
		labels := []string{
			"service", service,
			"method", method,
			"operation", operation,
			"grpc_code", grpcCode,
			"code", code,
		}
		s.metrics.DeltaUpDownCounter(ctx, metricGRPCServerInflight, -1,
			"service", service,
			"method", method,
			"operation", operation,
		)
		s.metrics.IncrementCounter(ctx, metricGRPCServerRequestsTotal, labels...)
		s.metrics.RecordHistogram(ctx, metricGRPCServerRequestSeconds, latency.Seconds(), labels...)
	}

	if !s.accessLog.Enabled {
		return
	}
	fields := []logx.Field{
		logx.String("service", service),
		logx.String("grpc_method", method),
		logx.String("grpc_code", grpcCode),
		logx.String("status_class", statusClass),
		logx.Bool("stream", stream),
		logx.Bool("retryable", errorx.RetryableOf(err)),
	}
	logx.LogAccess(logx.FromContextOr(ctx, s.logger), logx.AccessEvent{
		Side:       "server",
		Protocol:   "grpc",
		Component:  "grpc",
		Operation:  operation,
		StatusCode: status,
		Code:       code,
		Reason:     grpcCode,
		Latency:    latency,
		Err:        err,
		Fields:     fields,
	})
}

func splitFullMethod(operation string) (service string, method string) {
	operation = strings.TrimPrefix(operation, "/")
	if operation == "" {
		return "", ""
	}
	parts := strings.Split(operation, "/")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}
