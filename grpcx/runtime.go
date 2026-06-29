package grpcx

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"buf.build/go/protovalidate"
	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/timeout"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServerConfig defines the default Kernel gRPC server chain.
type ServerConfig struct {
	ServiceName string
	Logger      logx.Logger
	Metrics     metricsx.Manager
	Validator   protovalidate.Validator

	EnableOTel        bool
	EnableContext     bool
	EnableLogging     bool
	EnableMetrics     bool
	EnableValidation  bool
	EnableRecovery    bool
	ExtraUnary        []grpc.UnaryServerInterceptor
	ExtraStream       []grpc.StreamServerInterceptor
	ExtraServerOption []grpc.ServerOption
}

// DefaultServerConfig returns the standard gRPC server runtime profile.
func DefaultServerConfig(serviceName string) ServerConfig {
	return ServerConfig{
		ServiceName:      serviceName,
		EnableOTel:       true,
		EnableContext:    true,
		EnableLogging:    true,
		EnableMetrics:    true,
		EnableValidation: true,
		EnableRecovery:   true,
	}
}

// ServerOptions builds grpc.ServerOption values for Kernel services. It uses
// go-grpc-middleware recovery/protovalidate interceptors and otelgrpc stats
// handlers, while Kernel-specific context/log/metrics interceptors keep the
// business-facing API stable.
func ServerOptions(conf ServerConfig) []grpc.ServerOption {
	logger := conf.Logger
	if logger == nil {
		logger = logx.DefaultLogger().Named("grpcx")
	}
	metrics := metricsx.Ensure(conf.Metrics)
	unary := make([]grpc.UnaryServerInterceptor, 0, 8)
	stream := make([]grpc.StreamServerInterceptor, 0, 8)
	if conf.EnableContext {
		unary = append(unary, UnaryContextServerInterceptor())
		stream = append(stream, StreamContextServerInterceptor())
	}
	if conf.EnableValidation && conf.Validator != nil {
		unary = append(unary, UnaryProtovalidateServerInterceptor(conf.Validator))
		stream = append(stream, StreamProtovalidateServerInterceptor(conf.Validator))
	}
	if conf.EnableLogging {
		unary = append(unary, UnaryServerLoggingInterceptor(logger))
		stream = append(stream, StreamServerLoggingInterceptor(logger))
	}
	if conf.EnableMetrics {
		unary = append(unary, UnaryServerMetricsInterceptor(metrics))
		stream = append(stream, StreamServerMetricsInterceptor(metrics))
	}
	unary = append(unary, conf.ExtraUnary...)
	stream = append(stream, conf.ExtraStream...)
	if conf.EnableRecovery {
		unary = append(unary, recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(panicRecoveryHandler(logger))))
		stream = append(stream, recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(panicRecoveryHandler(logger))))
	}
	opts := make([]grpc.ServerOption, 0, 4+len(conf.ExtraServerOption))
	if conf.EnableOTel {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	if len(unary) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unary...))
	}
	if len(stream) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(stream...))
	}
	opts = append(opts, conf.ExtraServerOption...)
	return opts
}

// ClientConfig defines the default Kernel gRPC client chain.
type ClientConfig struct {
	ServiceName string
	Logger      logx.Logger
	Metrics     metricsx.Manager
	Timeout     time.Duration

	EnableOTel      bool
	EnableContext   bool
	EnableLogging   bool
	EnableMetrics   bool
	EnableTimeout   bool
	EnableRetry     bool
	RetryMax        uint
	RetryCodes      []codes.Code
	ExtraUnary      []grpc.UnaryClientInterceptor
	ExtraStream     []grpc.StreamClientInterceptor
	ExtraDialOption []grpc.DialOption
}

func DefaultClientConfig(serviceName string) ClientConfig {
	return ClientConfig{
		ServiceName:   serviceName,
		Timeout:       3 * time.Second,
		EnableOTel:    true,
		EnableContext: true,
		EnableLogging: true,
		EnableMetrics: true,
		EnableTimeout: true,
	}
}

// DialOptions builds grpc.DialOption values for Kernel gRPC clients. Add
// credentials/load-balancing options separately through ExtraDialOption or the
// caller's Dial call.
func DialOptions(conf ClientConfig) []grpc.DialOption {
	logger := conf.Logger
	if logger == nil {
		logger = logx.DefaultLogger().Named("grpcx.client")
	}
	metrics := metricsx.Ensure(conf.Metrics)
	unary := make([]grpc.UnaryClientInterceptor, 0, 8)
	stream := make([]grpc.StreamClientInterceptor, 0, 8)
	if conf.EnableContext {
		unary = append(unary, UnaryContextClientInterceptor())
		stream = append(stream, StreamContextClientInterceptor())
	}
	if conf.EnableLogging {
		unary = append(unary, UnaryClientLoggingInterceptor(logger))
		stream = append(stream, StreamClientLoggingInterceptor(logger))
	}
	if conf.EnableMetrics {
		unary = append(unary, UnaryClientMetricsInterceptor(metrics))
		stream = append(stream, StreamClientMetricsInterceptor(metrics))
	}
	if conf.EnableTimeout && conf.Timeout > 0 {
		unary = append(unary, timeout.UnaryClientInterceptor(conf.Timeout))
	}
	if conf.EnableRetry {
		codesToRetry := conf.RetryCodes
		if len(codesToRetry) == 0 {
			codesToRetry = []codes.Code{codes.Unavailable, codes.ResourceExhausted}
		}
		max := conf.RetryMax
		if max == 0 {
			max = 2
		}
		unary = append(unary, retry.UnaryClientInterceptor(retry.WithCodes(codesToRetry...), retry.WithMax(max)))
	}
	unary = append(unary, conf.ExtraUnary...)
	stream = append(stream, conf.ExtraStream...)
	opts := make([]grpc.DialOption, 0, 4+len(conf.ExtraDialOption))
	if conf.EnableOTel {
		opts = append(opts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}
	if len(unary) > 0 {
		opts = append(opts, grpc.WithChainUnaryInterceptor(unary...))
	}
	if len(stream) > 0 {
		opts = append(opts, grpc.WithChainStreamInterceptor(stream...))
	}
	opts = append(opts, conf.ExtraDialOption...)
	return opts
}

func panicRecoveryHandler(logger logx.Logger) func(any) error {
	return func(p any) error {
		logger.Error("grpc panic recovered", logx.Any("panic", p), logx.String("stack", string(debug.Stack())))
		return status.Error(codes.Internal, "internal server error")
	}
}

func UnaryServerLoggingInterceptor(logger logx.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logRPC(ctx, logger, "grpc server request", info.FullMethod, time.Since(start), err)
		return resp, err
	}
}

func StreamServerLoggingInterceptor(logger logx.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, stream)
		logRPC(stream.Context(), logger, "grpc server stream", info.FullMethod, time.Since(start), err)
		return err
	}
}

func UnaryClientLoggingInterceptor(logger logx.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		logRPC(ctx, logger, "grpc client request", method, time.Since(start), err)
		return err
	}
}

func StreamClientLoggingInterceptor(logger logx.Logger) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		cs, err := streamer(ctx, desc, cc, method, opts...)
		logRPC(ctx, logger, "grpc client stream", method, time.Since(start), err)
		return cs, err
	}
}

func logRPC(ctx context.Context, logger logx.Logger, msg, method string, dur time.Duration, err error) {
	if logger == nil {
		return
	}
	fields := []logx.Field{
		logx.String("method", method),
		logx.Duration("duration", dur),
		logx.String("request_id", contextx.RequestIDFromContext(ctx)),
		logx.String("trace_id", contextx.TraceIDFromContext(ctx)),
	}
	if err != nil {
		fields = append(fields, logx.String("code", status.Code(err).String()), logx.Err(err))
		logger.WithContext(ctx).Warn(msg, fields...)
		return
	}
	logger.WithContext(ctx).Info(msg, fields...)
}

func UnaryServerMetricsInterceptor(m metricsx.Manager) grpc.UnaryServerInterceptor {
	m.NewCounter("grpc_server_requests_total", "Total gRPC server unary requests")
	m.NewHistogram("grpc_server_request_seconds", "gRPC server unary request latency")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		recordRPCMetrics(ctx, m, "grpc_server", info.FullMethod, time.Since(start), err)
		return resp, err
	}
}

func StreamServerMetricsInterceptor(m metricsx.Manager) grpc.StreamServerInterceptor {
	m.NewCounter("grpc_server_stream_requests_total", "Total gRPC server stream requests")
	m.NewHistogram("grpc_server_stream_request_seconds", "gRPC server stream latency")
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, stream)
		recordRPCMetrics(stream.Context(), m, "grpc_server_stream", info.FullMethod, time.Since(start), err)
		return err
	}
}

func UnaryClientMetricsInterceptor(m metricsx.Manager) grpc.UnaryClientInterceptor {
	m.NewCounter("grpc_client_requests_total", "Total gRPC client unary requests")
	m.NewHistogram("grpc_client_request_seconds", "gRPC client unary request latency")
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		recordRPCMetrics(ctx, m, "grpc_client", method, time.Since(start), err)
		return err
	}
}

func StreamClientMetricsInterceptor(m metricsx.Manager) grpc.StreamClientInterceptor {
	m.NewCounter("grpc_client_stream_requests_total", "Total gRPC client stream requests")
	m.NewHistogram("grpc_client_stream_request_seconds", "gRPC client stream latency")
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		cs, err := streamer(ctx, desc, cc, method, opts...)
		recordRPCMetrics(ctx, m, "grpc_client_stream", method, time.Since(start), err)
		return cs, err
	}
}

func recordRPCMetrics(ctx context.Context, m metricsx.Manager, prefix, method string, dur time.Duration, err error) {
	code := codes.OK.String()
	if err != nil {
		code = status.Code(err).String()
	}
	m.IncrementCounter(ctx, prefix+"_requests_total", "method", method, "code", code)
	m.RecordHistogram(ctx, prefix+"_request_seconds", dur.Seconds(), "method", method, "code", code)
}

func formatPanic(p any) string { return fmt.Sprint(p) }
