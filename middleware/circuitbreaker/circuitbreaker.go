package circuitbreaker

import (
	"context"

	"github.com/aisphereio/kernel/errorx"
	internalbreaker "github.com/aisphereio/kernel/internal/circuitbreaker"
	"github.com/aisphereio/kernel/internal/group"
	"github.com/aisphereio/kernel/middleware"
	transport "github.com/aisphereio/kernel/transportx"
)

// ErrNotAllowed is request failed due to circuit breaker triggered.
var ErrNotAllowed = errorx.Unavailable("CIRCUIT_BREAKER_OPEN", "request failed due to circuit breaker triggered")

// CircuitBreaker is a circuit breaker.
type CircuitBreaker = internalbreaker.CircuitBreaker

// Option is circuit breaker option.
type Option func(*options)

// WithBreakerFactory configures a factory used to lazily create one circuit breaker per operation.
func WithBreakerFactory(factory func() CircuitBreaker) Option {
	return func(o *options) {
		if factory == nil {
			return
		}
		o.group = group.NewGroup(factory)
	}
}

type options struct {
	group *group.Group[CircuitBreaker]
}

// Client circuitbreaker middleware will return errBreakerTriggered when the circuit
// breaker is triggered and the request is rejected directly.
func Client(opts ...Option) middleware.Middleware {
	opt := &options{
		group: group.NewGroup(func() CircuitBreaker {
			return internalbreaker.NewBreaker()
		}),
	}
	for _, o := range opts {
		o(opt)
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			info, _ := transport.FromClientContext(ctx)
			breaker := opt.group.Get(info.Operation())
			if err := breaker.Allow(); err != nil {
				// rejected
				// NOTE: when client reject requests locally,
				// continue to add counter let the drop ratio higher.
				breaker.MarkFailed()
				return nil, ErrNotAllowed
			}
			// allowed
			reply, err := handler(ctx, req)
			if err != nil && (errorx.IsInternal(err) || errorx.IsUnavailable(err) || errorx.IsTimeout(err)) {
				breaker.MarkFailed()
			} else {
				breaker.MarkSuccess()
			}
			return reply, err
		}
	}
}
