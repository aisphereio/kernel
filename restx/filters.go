package restx

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/errorx"
	internalbreaker "github.com/aisphereio/kernel/internal/circuitbreaker"
	"github.com/aisphereio/kernel/internal/group"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/ratelimitx"
	"github.com/aisphereio/kernel/servicecontextx"
	httpx "github.com/aisphereio/kernel/transportx/http"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// RequestContextConfig configures request-scoped value injection.
type RequestContextConfig struct {
	Logger          logx.Logger
	Metrics         metricsx.Manager
	ServiceContext  *servicecontextx.Context
	RequestIDHeader string
	TraceIDHeader   string
}

// RequestContext injects request_id, trace_id, logx logger, metrics manager and
// optional ServiceContext into contextx. It is intentionally protocol-agnostic
// so generated Gateway/BFF logic can depend on contextx only.
func RequestContext(conf RequestContextConfig) httpx.FilterFunc {
	requestIDHeader := conf.RequestIDHeader
	if requestIDHeader == "" {
		requestIDHeader = "X-Request-Id"
	}
	traceIDHeader := conf.TraceIDHeader
	if traceIDHeader == "" {
		traceIDHeader = "X-Trace-Id"
	}
	logger := conf.Logger
	if logger == nil {
		logger = logx.DefaultLogger().Named("restx")
	}
	metrics := metricsx.Ensure(conf.Metrics)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			reqID := r.Header.Get(requestIDHeader)
			if reqID == "" {
				reqID = uuid.NewString()
				r.Header.Set(requestIDHeader, reqID)
			}
			traceID := r.Header.Get(traceIDHeader)
			if traceID == "" {
				if sc := oteltrace.SpanContextFromContext(ctx); sc.IsValid() {
					traceID = sc.TraceID().String()
					r.Header.Set(traceIDHeader, traceID)
				}
			}
			ctx = contextx.InjectRequestContext(ctx,
				contextx.WithRequestIDOption(reqID),
				contextx.WithTraceIDOption(traceID),
				contextx.WithLoggerOption(logger.WithContext(ctx).With(logx.String("request_id", reqID), logx.String("trace_id", traceID))),
			)
			ctx = metricsx.Inject(ctx, metrics)
			if conf.ServiceContext != nil {
				ctx = contextx.WithServiceContext(ctx, conf.ServiceContext)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Trace starts an OpenTelemetry span for HTTP requests and extracts incoming
// W3C trace context through otelhttp. It also makes the active trace id visible
// to downstream RequestContext.
func Trace(operation string) httpx.FilterFunc {
	if operation == "" {
		operation = "kernel.http"
	}
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, operation)
	}
}

// AccessLog records one structured log line per request using the request-bound
// contextx logger when available.
func AccessLog(logger logx.Logger) httpx.FilterFunc {
	if logger == nil {
		logger = logx.DefaultLogger().Named("restx.access")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			dur := time.Since(start)
			l := contextx.LoggerFromContextOr(r.Context(), logger).WithContext(r.Context())
			l.Info("http request",
				logx.String("method", r.Method),
				logx.String("path", r.URL.Path),
				logx.Int("status", rw.status),
				logx.Duration("duration", dur),
				logx.String("request_id", contextx.RequestIDFromContext(r.Context())),
				logx.String("trace_id", contextx.TraceIDFromContext(r.Context())),
			)
		})
	}
}

// Metrics records low-cardinality HTTP request metrics through metricsx. The
// instruments are lazily registered once; projects can also register them during
// startup for stricter control.
func Metrics(manager metricsx.Manager) httpx.FilterFunc {
	manager = metricsx.Ensure(manager)
	manager.NewCounter("http_requests_total", "Total HTTP requests")
	manager.NewHistogram("http_request_seconds", "HTTP request latency", metricsx.DefaultBuckets...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			status := strconv.Itoa(rw.status)
			manager.IncrementCounter(r.Context(), "http_requests_total",
				"method", r.Method,
				"status", status,
			)
			manager.RecordHistogram(r.Context(), "http_request_seconds", time.Since(start).Seconds(),
				"method", r.Method,
				"status", status,
			)
		})
	}
}

// Recover converts panics into a 500 response and logs the stack. It is the
// HTTP-filter counterpart of middleware/recovery for raw HTTP/BFF handlers.
func Recover() httpx.FilterFunc {
	logger := logx.DefaultLogger().Named("restx.recover")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						logx.Any("panic", rec),
						logx.String("path", r.URL.Path),
						logx.String("stack", string(debug.Stack())),
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBytes caps request body size before JSON/form decoders read it.
func MaxBytes(limit int64) httpx.FilterFunc {
	if limit <= 0 {
		limit = 32 << 20
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxConns bounds concurrent in-flight HTTP requests.
func MaxConns(limit int) httpx.FilterFunc {
	if limit <= 0 {
		limit = 1024
	}
	sem := make(chan struct{}, limit)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			}
		})
	}
}

// Timeout attaches a request context timeout. Handlers should respect
// r.Context().Done(); this filter avoids transport replacement and stays
// compatible with transportx/http's own timeout option.
func Timeout(timeout time.Duration) httpx.FilterFunc {
	if timeout <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Breaker rejects requests when the route-level circuit breaker is open.
func Breaker() httpx.FilterFunc {
	breakers := group.NewGroup(func() internalbreaker.CircuitBreaker { return internalbreaker.NewBreaker() })
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Method + " " + r.URL.Path
			breaker := breakers.Get(key)
			if err := breaker.Allow(); err != nil {
				breaker.MarkFailed()
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			if rw.status >= 500 || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
				breaker.MarkFailed()
				return
			}
			breaker.MarkSuccess()
		})
	}
}

// Shedding applies Kernel's public rate-limit provider contract as an HTTP
// self-protection filter.
func Shedding(policies ...ratelimitx.Policy) httpx.FilterFunc {
	var policy ratelimitx.Policy
	if len(policies) > 0 {
		policy = policies[0]
	}
	return SheddingWithProvider(policy, nil)
}

// SheddingWithProvider applies a caller-supplied rate-limit provider. A nil
// provider uses the in-memory provider for local self-protection.
func SheddingWithProvider(policy ratelimitx.Policy, provider ratelimitx.Provider) httpx.FilterFunc {
	if !policy.Enabled {
		policy = ratelimitx.Policy{
			Enabled: true,
			Name:    "restx.shedding",
			Backend: ratelimitx.BackendMemory,
			Scope:   ratelimitx.ScopeLocalInstance,
			Key:     ratelimitx.KeyOperation,
			QPS:     1000,
			Burst:   1000,
		}
	}
	if provider == nil {
		provider = ratelimitx.NewMemoryProvider()
	}
	limiter, limiterErr := provider.NewLimiter(policy)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiterErr != nil {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}
			key := r.Method + " " + r.URL.Path
			decision, err := limiter.Allow(r.Context(), key, 1)
			if err != nil || !decision.Allowed {
				if decision.RetryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(decision.RetryAfter.Seconds())))
				}
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Gunzip transparently decompresses gzip request bodies.
func Gunzip() httpx.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			defer zr.Close()
			r.Body = struct {
				io.Reader
				io.Closer
			}{Reader: zr, Closer: r.Body}
			r.Header.Del("Content-Encoding")
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds conservative browser-facing security headers.
func SecurityHeaders() httpx.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			setDefault(h, "X-Content-Type-Options", "nosniff")
			setDefault(h, "X-Frame-Options", "DENY")
			setDefault(h, "Referrer-Policy", "strict-origin-when-cross-origin")
			setDefault(h, "Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			next.ServeHTTP(w, r)
		})
	}
}

// SanitizeHeaders rejects requests with CR/LF in header values. net/http
// protects response splitting in normal writes, but this early check gives
// Gateway/BFF code a single place to fail closed before proxying upstream.
func SanitizeHeaders() httpx.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for name, values := range r.Header {
				for _, v := range values {
					if strings.ContainsAny(v, "\r\n") {
						http.Error(w, "invalid header "+name, http.StatusBadRequest)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig configures the CORS filter.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// CORS handles simple and preflight CORS requests.
func CORS(conf CORSConfig) httpx.FilterFunc {
	allowedOrigins := map[string]struct{}{}
	allowAll := false
	for _, origin := range conf.AllowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		allowedOrigins[origin] = struct{}{}
	}
	methods := strings.Join(defaultStrings(conf.AllowedMethods, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}), ", ")
	headers := strings.Join(defaultStrings(conf.AllowedHeaders, []string{"Content-Type", "Authorization", "X-CSRF-Token", "X-Request-Id"}), ", ")
	exposed := strings.Join(conf.ExposedHeaders, ", ")
	maxAge := int(conf.MaxAge.Seconds())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll && !conf.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allowedOrigins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				if conf.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if exposed != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposed)
				}
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				if maxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  atomic.Bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wrote.CompareAndSwap(false, true) {
		r.status = status
		r.ResponseWriter.WriteHeader(status)
	}
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if !r.wrote.Load() {
		r.WriteHeader(r.status)
	}
	return r.ResponseWriter.Write(data)
}

func statusAsError(status int) error {
	if status >= 500 {
		return errorx.Unavailable("HTTP_UPSTREAM_UNAVAILABLE", http.StatusText(status))
	}
	if status == http.StatusTooManyRequests {
		return errorx.TooManyRequests("HTTP_RATE_LIMITED", http.StatusText(status))
	}
	return nil
}

func setDefault(h http.Header, key, value string) {
	if h.Get(key) == "" {
		h.Set(key, value)
	}
}

func defaultStrings(got, fallback []string) []string {
	if len(got) == 0 {
		return fallback
	}
	return got
}
