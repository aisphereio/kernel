// Package timeout provides transport-agnostic timeout middleware.
package timeout

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
)

var ErrTimeout = errorx.Timeout("REQUEST_TIMEOUT", "request deadline exceeded")

type Option func(*options)

type options struct{ duration time.Duration }

func WithDuration(d time.Duration) Option { return func(o *options) { o.duration = d } }

func Server(opts ...Option) middleware.Middleware { return apply(opts...) }
func Client(opts ...Option) middleware.Middleware { return apply(opts...) }

func apply(opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if o.duration <= 0 {
				return next(ctx, req)
			}
			cctx, cancel := context.WithTimeout(ctx, o.duration)
			defer cancel()
			reply, err := next(cctx, req)
			if err != nil {
				return reply, err
			}
			if cctx.Err() == context.DeadlineExceeded {
				return reply, ErrTimeout
			}
			return reply, nil
		}
	}
}
