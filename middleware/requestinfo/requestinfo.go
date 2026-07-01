// Package requestinfo resolves requestx.Info once near the front of the middleware chain.
package requestinfo

import (
	"context"

	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/requestx"
)

type Option func(*options)

type options struct{ resolvers []requestx.Resolver }

func WithResolver(r requestx.Resolver) Option {
	return func(o *options) {
		if r != nil {
			o.resolvers = append(o.resolvers, r)
		}
	}
}

// Server attaches normalized request metadata to ctx.
func Server(opts ...Option) middleware.Middleware { return build(requestx.DirectionServer, opts...) }

// Client attaches normalized outbound metadata to ctx.
func Client(opts ...Option) middleware.Middleware { return build(requestx.DirectionClient, opts...) }

func build(direction requestx.Direction, opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			base, _ := requestx.FromTransport(ctx, direction)
			base = requestx.EnrichFromContext(ctx, base)
			operation := base.Operation
			merged := base
			for _, resolver := range o.resolvers {
				info, ok, err := resolver(ctx, operation, req)
				if err != nil {
					return nil, err
				}
				if ok {
					merged = merged.Merge(info)
				}
			}
			ctx = requestx.NewContext(ctx, requestx.EnrichFromContext(ctx, merged))
			return next(ctx, req)
		}
	}
}
