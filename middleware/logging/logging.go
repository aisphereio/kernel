package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/middleware"
	transport "github.com/aisphereio/kernel/transportx"
)

// Redacter defines how to log an object.
type Redacter interface {
	Redact() string
}

// Server is a server logging middleware.
func Server(logger logx.Logger) middleware.Middleware {
	return access(logger, "server", transport.FromServerContext)
}

// Client is a client logging middleware.
func Client(logger logx.Logger) middleware.Middleware {
	return access(logger, "client", transport.FromClientContext)
}

type transportExtractor func(context.Context) (transport.Transporter, bool)

func access(logger logx.Logger, side string, extract transportExtractor) middleware.Middleware {
	if logger == nil {
		logger = logx.Noop()
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			var kind, operation string
			startTime := time.Now()
			if info, ok := extract(ctx); ok {
				kind = info.Kind().String()
				operation = info.Operation()
			}
			reply, err = handler(ctx, req)

			fields := []logx.Field{
				logx.Event("rpc_access"),
				logx.String("side", side),
				logx.String("component", kind),
				logx.String("operation", operation),
				logx.String("args", extractArgs(req)),
				logx.Duration("latency", time.Since(startTime)),
			}
			fields = append(fields, errorFields(err)...)

			l := logx.FromContextOr(ctx, logger)
			if err != nil {
				l.Error(side+" request failed", fields...)
			} else {
				l.Info(side+" request completed", fields...)
			}
			return
		}
	}
}

// extractArgs returns the string of the req.
func extractArgs(req any) string {
	if redacter, ok := req.(Redacter); ok {
		return redacter.Redact()
	}
	if stringer, ok := req.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%+v", req)
}

func errorFields(err error) []logx.Field {
	if err == nil {
		return nil
	}
	fields := []logx.Field{logx.Err(err)}
	for k, v := range errorx.Fields(err) {
		fields = append(fields, logx.Any(k, v))
	}
	return fields
}
