package logging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/transport"
)

// Redacter defines how to log an object
type Redacter interface {
	Redact() string
}

// Server is an server logging middleware.
func Server(logger *slog.Logger) middleware.Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			var (
				kind      string
				operation string
			)
			startTime := time.Now()
			if info, ok := transport.FromServerContext(ctx); ok {
				kind = info.Kind().String()
				operation = info.Operation()
			}
			reply, err = handler(ctx, req)
			level, stack := extractError(err)
			attrs := []slog.Attr{
				slog.String("kind", "server"),
				slog.String("component", kind),
				slog.String("operation", operation),
				slog.String("args", extractArgs(req)),
				slog.Float64("latency", time.Since(startTime).Seconds()),
			}
			attrs = append(attrs, errorAttrs(err)...)
			if err != nil && stack != "" {
				attrs = append(attrs, slog.String("stack", stack))
			}
			logger.LogAttrs(ctx, level, "server request", attrs...)
			return
		}
	}
}

// Client is a client logging middleware.
func Client(logger *slog.Logger) middleware.Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			var (
				kind      string
				operation string
			)
			startTime := time.Now()
			if info, ok := transport.FromClientContext(ctx); ok {
				kind = info.Kind().String()
				operation = info.Operation()
			}
			reply, err = handler(ctx, req)
			level, stack := extractError(err)
			attrs := []slog.Attr{
				slog.String("kind", "client"),
				slog.String("component", kind),
				slog.String("operation", operation),
				slog.String("args", extractArgs(req)),
				slog.Float64("latency", time.Since(startTime).Seconds()),
			}
			attrs = append(attrs, errorAttrs(err)...)
			if err != nil && stack != "" {
				attrs = append(attrs, slog.String("stack", stack))
			}
			logger.LogAttrs(ctx, level, "client request", attrs...)
			return
		}
	}
}

// extractArgs returns the string of the req
func extractArgs(req any) string {
	if redacter, ok := req.(Redacter); ok {
		return redacter.Redact()
	}
	if stringer, ok := req.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%+v", req)
}

// extractError returns the level and stack to attach for err.
func extractError(err error) (slog.Level, string) {
	if err != nil {
		return slog.LevelError, fmt.Sprintf("%+v", err)
	}
	return slog.LevelInfo, ""
}

func errorAttrs(err error) []slog.Attr {
	fields := errorx.Fields(err)
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}
	return attrs
}
