package polaris

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/middleware/ratelimit"
	"github.com/aisphereio/kernel/transport"
	"github.com/aisphereio/kernel/transport/http"

	"github.com/polarismesh/polaris-go/pkg/model"
)

// ErrLimitExceed is service unavailable due to rate limit exceeded.
var (
	ErrLimitExceed = errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "service unavailable due to rate limit exceeded")
)

// Ratelimit Request rate limit middleware
func Ratelimit(l Limiter) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				var args []model.Argument
				headers := tr.RequestHeader()
				// handle header
				for _, header := range headers.Keys() {
					args = append(args, model.BuildHeaderArgument(header, headers.Get(header)))
				}
				// handle http
				if ht, ok := tr.(*http.Transport); ok {
					// url query
					for key, values := range ht.Request().URL.Query() {
						args = append(args, model.BuildQueryArgument(key, strings.Join(values, ",")))
					}
				}
				done, e := l.Allow(tr.Operation(), args...)
				if e != nil {
					// rejected
					return nil, ErrLimitExceed
				}
				// allowed
				reply, err = handler(ctx, req)
				done(ratelimit.DoneInfo{Err: err})
				return
			}
			return reply, nil
		}
	}
}
