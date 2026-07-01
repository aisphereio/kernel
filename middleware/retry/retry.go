// Package retry provides client-side retry middleware for transient failures.
package retry

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
)

type Option func(*options)

type options struct {
	maxAttempts int
	backoff     time.Duration
	retryable   func(error) bool
}

func WithMaxAttempts(n int) Option       { return func(o *options) { o.maxAttempts = n } }
func WithBackoff(d time.Duration) Option { return func(o *options) { o.backoff = d } }
func WithRetryable(fn func(error) bool) Option {
	return func(o *options) {
		if fn != nil {
			o.retryable = fn
		}
	}
}

func Client(opts ...Option) middleware.Middleware {
	o := &options{maxAttempts: 2, backoff: 25 * time.Millisecond, retryable: defaultRetryable}
	for _, opt := range opts {
		opt(o)
	}
	if o.maxAttempts < 1 {
		o.maxAttempts = 1
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			var reply any
			var err error
			for attempt := 1; attempt <= o.maxAttempts; attempt++ {
				reply, err = next(ctx, req)
				if err == nil || attempt == o.maxAttempts || !o.retryable(err) {
					return reply, err
				}
				if o.backoff > 0 {
					timer := time.NewTimer(o.backoff)
					select {
					case <-ctx.Done():
						timer.Stop()
						return reply, errorx.Timeout("RETRY_CONTEXT_DONE", "retry stopped because context is done")
					case <-timer.C:
					}
				}
			}
			return reply, err
		}
	}
}

func defaultRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errorx.RetryableOf(err) {
		return true
	}
	return errorx.IsUnavailable(err) || errorx.IsTimeout(err) || errorx.IsInternal(err)
}
