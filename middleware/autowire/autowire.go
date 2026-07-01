// Package autowire assembles Kernel's default middleware pipelines.
//
// The package keeps transport setup boring: services opt into providers and
// resolvers, while Kernel fixes the middleware order consistently for HTTP and
// gRPC servers/clients.
package autowire

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/admissionx"
	rootauthn "github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
	accessmw "github.com/aisphereio/kernel/middleware/access"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/middleware/circuitbreaker"
	"github.com/aisphereio/kernel/middleware/ctxinject"
	metamw "github.com/aisphereio/kernel/middleware/metadata"
	requestinfomw "github.com/aisphereio/kernel/middleware/requestinfo"
	"github.com/aisphereio/kernel/middleware/retry"
	timeoutmw "github.com/aisphereio/kernel/middleware/timeout"
	"github.com/aisphereio/kernel/ratelimitx"
	"github.com/aisphereio/kernel/requestx"
)

// RateLimitKeyFunc derives a limiter key from request context and payload.
type RateLimitKeyFunc func(ctx context.Context, req any) string

// ServerOption configures the default server middleware pipeline.
type ServerOption func(*serverOptions)

type serverOptions struct {
	before []middleware.Middleware
	after  []middleware.Middleware

	timeout time.Duration

	ctxinject []ctxinject.Option

	requestInfoResolvers []requestx.Resolver

	metadataEnabled bool
	metadataOptions []metamw.Option

	authnEnabled        bool
	authenticator       rootauthn.Authenticator
	credentialExtractor authnmw.CredentialExtractor
	allowAnonymous      bool
	authnOptions        []authnmw.Option

	rateLimitEnabled  bool
	rateLimitPolicy   ratelimitx.Policy
	rateLimitProvider ratelimitx.Provider
	rateLimitKey      RateLimitKeyFunc

	accessEnabled bool
	guard         accessx.Guard
	resolver      accessmw.Resolver

	admission admissionx.Chain
}

// WithBefore prepends middleware before Kernel's default chain. Use sparingly;
// recovery usually belongs here when it is not owned by the transport.
func WithBefore(m ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) { o.before = append(o.before, m...) }
}

// WithAfter appends middleware after Kernel's default chain.
func WithAfter(m ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) { o.after = append(o.after, m...) }
}

// WithTimeout adds a server request timeout near the front of the chain.
func WithTimeout(d time.Duration) ServerOption { return func(o *serverOptions) { o.timeout = d } }

// WithContextInjection configures ctxinject.Server. Context injection is always
// enabled because later authn/authz/audit/logging stages depend on it.
func WithContextInjection(opts ...ctxinject.Option) ServerOption {
	return func(o *serverOptions) { o.ctxinject = append(o.ctxinject, opts...) }
}

// WithRequestInfoResolver adds a request metadata resolver. Generated code should provide this from proto access policy.
func WithRequestInfoResolver(resolver requestx.Resolver) ServerOption {
	return func(o *serverOptions) {
		if resolver != nil {
			o.requestInfoResolvers = append(o.requestInfoResolvers, resolver)
		}
	}
}

// WithMetadataPropagation enables Kernel metadata extraction on the server side.
func WithMetadataPropagation(opts ...metamw.Option) ServerOption {
	return func(o *serverOptions) {
		o.metadataEnabled = true
		o.metadataOptions = append(o.metadataOptions, opts...)
	}
}

// WithAuthn enables authentication middleware and syncs the resulting principal
// into authn and contextx contexts. Extra authn middleware options are applied
// after autowire's first-class options.
func WithAuthn(authenticator rootauthn.Authenticator, opts ...authnmw.Option) ServerOption {
	return func(o *serverOptions) {
		o.authnEnabled = true
		o.authenticator = authenticator
		o.authnOptions = append(o.authnOptions, opts...)
	}
}

// WithCredentialExtractor configures authn credential extraction.
func WithCredentialExtractor(extractor authnmw.CredentialExtractor) ServerOption {
	return func(o *serverOptions) { o.authnEnabled = true; o.credentialExtractor = extractor }
}

// WithAllowAnonymous enables anonymous requests in authn middleware.
func WithAllowAnonymous(allow bool) ServerOption {
	return func(o *serverOptions) { o.authnEnabled = true; o.allowAnonymous = allow }
}

// WithRateLimitPolicy enables policy-driven server-side rate limiting.
func WithRateLimitPolicy(policy ratelimitx.Policy, provider ratelimitx.Provider, key RateLimitKeyFunc) ServerOption {
	return func(o *serverOptions) {
		o.rateLimitEnabled = true
		o.rateLimitPolicy = policy
		o.rateLimitProvider = provider
		o.rateLimitKey = key
	}
}

// WithAccess enables authorization and audit middleware.
func WithAccess(guard accessx.Guard, resolver accessmw.Resolver) ServerOption {
	return func(o *serverOptions) { o.accessEnabled = true; o.guard = guard; o.resolver = resolver }
}

// WithAdmission enables Kubernetes-style mutating/validating admission before business logic.
func WithAdmission(chain admissionx.Chain) ServerOption {
	return func(o *serverOptions) { o.admission = chain }
}

// Server builds Kernel's default server pipeline in this order:
//
// custom before -> timeout -> ctx inject -> request info -> metadata -> authn -> rate limit -> authz/audit -> admission -> custom after
func Server(opts ...ServerOption) []middleware.Middleware {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}
	chain := make([]middleware.Middleware, 0, len(o.before)+len(o.after)+8)
	chain = append(chain, o.before...)
	if o.timeout > 0 {
		chain = append(chain, timeoutmw.Server(timeoutmw.WithDuration(o.timeout)))
	}
	chain = append(chain, ctxinject.Server(o.ctxinject...))
	chain = append(chain, requestinfomw.Server(requestInfoOptions(o.requestInfoResolvers)...))
	if o.metadataEnabled {
		chain = append(chain, metamw.Server(o.metadataOptions...))
	}
	if o.authnEnabled {
		authnOpts := []authnmw.Option{authnmw.WithAuthenticator(o.authenticator), authnmw.WithAllowAnonymous(o.allowAnonymous)}
		if o.credentialExtractor != nil {
			authnOpts = append(authnOpts, authnmw.WithCredentialExtractor(o.credentialExtractor))
		}
		authnOpts = append(authnOpts, o.authnOptions...)
		chain = append(chain, authnmw.Server(authnOpts...))
	}
	if o.rateLimitEnabled {
		chain = append(chain, rateLimitMiddleware(o.rateLimitPolicy, o.rateLimitProvider, o.rateLimitKey))
	}
	if o.accessEnabled {
		chain = append(chain, accessmw.Server(o.guard, accessmw.WithResolver(o.resolver)))
	}
	if !o.admission.Empty() {
		chain = append(chain, admissionx.Middleware(o.admission))
	}
	chain = append(chain, o.after...)
	return chain
}

// ClientOption configures the default client middleware pipeline.
type ClientOption func(*clientOptions)

type clientOptions struct {
	before []middleware.Middleware
	after  []middleware.Middleware

	requestInfoResolvers []requestx.Resolver

	metadataEnabled bool
	metadataOptions []metamw.Option
	timeout         time.Duration

	rateLimitEnabled  bool
	rateLimitPolicy   ratelimitx.Policy
	rateLimitProvider ratelimitx.Provider
	rateLimitKey      RateLimitKeyFunc

	circuitBreakerEnabled bool
	breakerFactory        func() circuitbreaker.CircuitBreaker

	retryEnabled bool
	retryOptions []retry.Option
}

// WithClientBefore prepends client middleware.
func WithClientBefore(m ...middleware.Middleware) ClientOption {
	return func(o *clientOptions) { o.before = append(o.before, m...) }
}

// WithClientAfter appends client middleware.
func WithClientAfter(m ...middleware.Middleware) ClientOption {
	return func(o *clientOptions) { o.after = append(o.after, m...) }
}

// WithClientRequestInfoResolver adds an outbound request metadata resolver.
func WithClientRequestInfoResolver(resolver requestx.Resolver) ClientOption {
	return func(o *clientOptions) {
		if resolver != nil {
			o.requestInfoResolvers = append(o.requestInfoResolvers, resolver)
		}
	}
}

// WithClientMetadataPropagation enables outbound metadata propagation.
func WithClientMetadataPropagation(opts ...metamw.Option) ClientOption {
	return func(o *clientOptions) {
		o.metadataEnabled = true
		o.metadataOptions = append(o.metadataOptions, opts...)
	}
}

// WithClientTimeout adds a per-downstream client timeout.
func WithClientTimeout(d time.Duration) ClientOption { return func(o *clientOptions) { o.timeout = d } }

// WithClientRateLimitPolicy enables client-side outbound rate limiting.
func WithClientRateLimitPolicy(policy ratelimitx.Policy, provider ratelimitx.Provider, key RateLimitKeyFunc) ClientOption {
	return func(o *clientOptions) {
		o.rateLimitEnabled = true
		o.rateLimitPolicy = policy
		o.rateLimitProvider = provider
		o.rateLimitKey = key
	}
}

// WithCircuitBreaker enables client-side circuit breaker middleware.
func WithCircuitBreaker() ClientOption {
	return func(o *clientOptions) { o.circuitBreakerEnabled = true }
}

// WithCircuitBreakerFactory enables client-side circuit breaker middleware with
// a custom factory. Tests and demos can inject deterministic breakers.
func WithCircuitBreakerFactory(factory func() circuitbreaker.CircuitBreaker) ClientOption {
	return func(o *clientOptions) { o.circuitBreakerEnabled = true; o.breakerFactory = factory }
}

// WithRetry enables client retries for retryable/transient errors.
func WithRetry(opts ...retry.Option) ClientOption {
	return func(o *clientOptions) { o.retryEnabled = true; o.retryOptions = append(o.retryOptions, opts...) }
}

// Client builds Kernel's default client pipeline in this order:
//
// custom before -> request info -> metadata -> timeout -> rate limit -> circuit breaker -> retry -> custom after
func Client(opts ...ClientOption) []middleware.Middleware {
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}
	chain := make([]middleware.Middleware, 0, len(o.before)+len(o.after)+6)
	chain = append(chain, o.before...)
	chain = append(chain, requestinfomw.Client(requestInfoOptions(o.requestInfoResolvers)...))
	if o.metadataEnabled {
		chain = append(chain, metamw.Client(o.metadataOptions...))
	}
	if o.timeout > 0 {
		chain = append(chain, timeoutmw.Client(timeoutmw.WithDuration(o.timeout)))
	}
	if o.rateLimitEnabled {
		chain = append(chain, rateLimitMiddleware(o.rateLimitPolicy, o.rateLimitProvider, o.rateLimitKey))
	}
	if o.circuitBreakerEnabled {
		if o.breakerFactory != nil {
			chain = append(chain, circuitbreaker.Client(circuitbreaker.WithBreakerFactory(o.breakerFactory)))
		} else {
			chain = append(chain, circuitbreaker.Client())
		}
	}
	if o.retryEnabled {
		chain = append(chain, retry.Client(o.retryOptions...))
	}
	chain = append(chain, o.after...)
	return chain
}

func rateLimitMiddleware(policy ratelimitx.Policy, provider ratelimitx.Provider, keyFn RateLimitKeyFunc) middleware.Middleware {
	policy = policy.Normalize()
	if provider == nil {
		provider = ratelimitx.NewMemoryProvider()
	}
	limiter, err := provider.NewLimiter(policy)
	if err != nil {
		return rejectMiddleware(errorx.Internal("RATE_LIMIT_CONFIG_INVALID", err.Error()))
	}
	if keyFn == nil {
		keyFn = operationKey
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			key := keyFn(ctx, req)
			decision, err := limiter.Allow(ctx, key, 1)
			if err != nil {
				if policy.FailMode == ratelimitx.FailOpen {
					return next(ctx, req)
				}
				return nil, errorx.Unavailable("RATE_LIMIT_PROVIDER_UNAVAILABLE", "rate limit provider unavailable", errorx.WithCause(err))
			}
			if !decision.Allowed {
				return nil, ratelimitx.ErrLimited
			}
			return next(ctx, req)
		}
	}
}

func rejectMiddleware(err error) middleware.Middleware {
	return func(middleware.Handler) middleware.Handler {
		return func(context.Context, any) (any, error) { return nil, err }
	}
}

func operationKey(ctx context.Context, _ any) string {
	return requestx.OperationKey(ctx)
}

func requestInfoOptions(resolvers []requestx.Resolver) []requestinfomw.Option {
	opts := make([]requestinfomw.Option, 0, len(resolvers))
	for _, resolver := range resolvers {
		opts = append(opts, requestinfomw.WithResolver(resolver))
	}
	return opts
}
