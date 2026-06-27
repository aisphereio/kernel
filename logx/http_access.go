package logx

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var errHijackNotSupported = errors.New("response writer does not support hijacking")

type PrincipalResolver func(*http.Request) []Field
type RouteResolver func(*http.Request) string

type HTTPAccessOption func(*httpAccessOptions)

type httpAccessOptions struct {
	principalResolver PrincipalResolver
	routeResolver     RouteResolver
	requestIDHeader   string
}

func WithPrincipalResolver(fn PrincipalResolver) HTTPAccessOption {
	return func(o *httpAccessOptions) { o.principalResolver = fn }
}

func WithRouteResolver(fn RouteResolver) HTTPAccessOption {
	return func(o *httpAccessOptions) { o.routeResolver = fn }
}

func WithRequestIDHeader(header string) HTTPAccessOption {
	return func(o *httpAccessOptions) { o.requestIDHeader = header }
}

func HTTPAccessLog(base Logger, cfg AccessLogConfig, opts ...HTTPAccessOption) func(http.Handler) http.Handler {
	if base == nil {
		base = Noop()
	}
	if cfg.RequestIDHeader == "" {
		cfg.RequestIDHeader = "X-Request-ID"
	}
	if cfg.RouteHeader == "" {
		cfg.RouteHeader = "X-Route-Pattern"
	}
	options := &httpAccessOptions{requestIDHeader: cfg.RequestIDHeader}
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, path := range cfg.SkipPaths {
		if path != "" {
			skip[path] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := r.Header.Get(options.requestIDHeader)
			if requestID == "" {
				requestID = newRequestID()
			}
			w.Header().Set(options.requestIDHeader, requestID)

			ctx := r.Context()
			ctxFields := []Field{
				String("request_id", requestID),
				String("client_ip", clientIP(r)),
			}
			ctxFields = append(ctxFields, traceFieldsFromHTTPRequest(r)...)
			if options.principalResolver != nil {
				ctxFields = append(ctxFields, options.principalResolver(r)...)
			}
			ctx = Inject(ctx, base, ctxFields...)
			r = r.WithContext(ctx)

			rw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			if !cfg.Enabled || shouldSkipAccess(skip, r.URL.Path) {
				return
			}

			latency := time.Since(start)
			route := ""
			if options.routeResolver != nil {
				route = options.routeResolver(r)
			}
			if route == "" {
				route = r.Header.Get(cfg.RouteHeader)
			}
			fields := []Field{
				Event("access"),
				String("side", "server"),
				String("protocol", "http"),
				String("method", r.Method),
				String("path", r.URL.Path),
				String("route", route),
				String("query", r.URL.RawQuery),
				Int("status", rw.status),
				String("status_class", strconv.Itoa(rw.status/100)+"xx"),
				Duration("latency", latency),
				Int64("latency_ms", latency.Milliseconds()),
				Int64("request_size", requestSize(r)),
				Int("response_size", rw.bytes),
			}
			if cfg.LogUserAgent {
				fields = append(fields, String("user_agent", r.UserAgent()))
			}
			if cfg.LogReferer {
				fields = append(fields, String("referer", r.Referer()))
			}

			logger := FromContextOr(ctx, base)
			msg := "http request completed"
			switch {
			case rw.status >= 500:
				logger.Error(msg, fields...)
			case rw.status >= 400:
				logger.Warn(msg, fields...)
			case cfg.SlowThreshold > 0 && latency >= cfg.SlowThreshold:
				logger.Warn("http request slow", fields...)
			default:
				logger.Info(msg, fields...)
			}
		})
	}
}

func shouldSkipAccess(skip map[string]struct{}, path string) bool {
	if _, ok := skip[path]; ok {
		return true
	}
	for pattern := range skip {
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errHijackNotSupported
	}
	return h.Hijack()
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func requestSize(r *http.Request) int64 {
	if r.ContentLength > 0 {
		return r.ContentLength
	}
	return 0
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// traceFieldsFromHTTPRequest extracts W3C traceparent without importing OTel.
// An OTel-based extractor can be added in logx/otelx when the app already uses OTel.
func traceFieldsFromHTTPRequest(r *http.Request) []Field {
	traceparent := r.Header.Get("traceparent")
	if traceparent == "" {
		return nil
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) < 4 {
		return nil
	}
	traceID := parts[1]
	spanID := parts[2]
	if len(traceID) != 32 || len(spanID) != 16 {
		return nil
	}
	return []Field{String("trace_id", traceID), String("span_id", spanID)}
}

func WithHTTPContext(ctx context.Context, logger Logger, r *http.Request, fields ...Field) context.Context {
	if r != nil {
		fields = append(fields, traceFieldsFromHTTPRequest(r)...)
	}
	return Inject(ctx, logger, fields...)
}
