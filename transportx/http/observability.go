package http

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

var errMetricsHijackNotSupported = errors.New("response writer does not support hijacking")

const (
	metricHTTPServerRequestsTotal  = "kernel_http_server_requests_total"
	metricHTTPServerRequestSeconds = "kernel_http_server_request_duration_seconds"
	metricHTTPServerInflight       = "kernel_http_server_inflight_requests"
	metricHTTPServerHandlerErrors  = "kernel_http_server_handler_errors_total"
)

func (s *Server) registerMetrics() {
	if s.metrics == nil {
		return
	}
	s.metrics.NewCounter(metricHTTPServerRequestsTotal, "Total HTTP server requests")
	s.metrics.NewHistogram(metricHTTPServerRequestSeconds, "HTTP server request latency in seconds", 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
	s.metrics.NewUpDownCounter(metricHTTPServerInflight, "Current in-flight HTTP server requests")
	s.metrics.NewCounter(metricHTTPServerHandlerErrors, "Total HTTP handler errors returned before encoding")
}

func (s *Server) metricsMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.metrics == nil {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			route := routePattern(r)
			if route == "" {
				route = r.URL.Path
			}
			s.metrics.DeltaUpDownCounter(r.Context(), metricHTTPServerInflight, 1, "method", r.Method, "route", route)
			defer s.metrics.DeltaUpDownCounter(r.Context(), metricHTTPServerInflight, -1, "method", r.Method, "route", route)

			rw := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			status := strconv.Itoa(rw.status)
			statusClass := strconv.Itoa(rw.status/100) + "xx"
			labels := []string{"method", r.Method, "route", route, "status", status, "status_class", statusClass}
			s.metrics.IncrementCounter(r.Context(), metricHTTPServerRequestsTotal, labels...)
			s.metrics.RecordHistogram(r.Context(), metricHTTPServerRequestSeconds, time.Since(start).Seconds(), labels...)
		})
	}
}

func (s *Server) observeHandlerError(ctx context.Context, req *http.Request, route string, err error) {
	if err == nil {
		return
	}
	if route == "" && req != nil {
		route = routePattern(req)
	}
	if route == "" && req != nil {
		route = req.URL.Path
	}
	method := ""
	path := ""
	if req != nil {
		method = req.Method
		path = req.URL.Path
	}
	status := errorx.HTTPStatusOf(err)
	code := errorx.CodeOf(err).String()
	fields := []logx.Field{
		logx.Event("handler_error"),
		logx.String("protocol", "http"),
		logx.String("method", method),
		logx.String("path", path),
		logx.String("route", route),
		logx.Int("status", status),
		logx.String("status_class", strconv.Itoa(status/100)+"xx"),
		logx.String("code", code),
		logx.String("grpc_code", errorx.GRPCCodeOf(err)),
		logx.Bool("retryable", errorx.RetryableOf(err)),
		logx.Err(err),
	}
	for k, v := range errorx.SafeMetadataOf(err) {
		fields = append(fields, logx.Any("error_metadata."+k, v))
	}
	logger := logx.FromContextOr(ctx, s.logger)
	switch {
	case status >= 500:
		logger.Error("http handler failed", fields...)
	case status >= 400:
		logger.Warn("http handler rejected request", fields...)
	default:
		logger.Info("http handler returned error", fields...)
	}
	if s.metricsEnabled && s.metrics != nil {
		s.metrics.IncrementCounter(ctx, metricHTTPServerHandlerErrors,
			"method", method,
			"route", route,
			"status", strconv.Itoa(status),
			"status_class", strconv.Itoa(status/100)+"xx",
			"code", code,
		)
	}
}

func routePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	if route := mux.CurrentRoute(r); route != nil {
		if pattern, err := route.GetPathTemplate(); err == nil {
			return pattern
		}
	}
	return ""
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *metricsResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *metricsResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *metricsResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *metricsResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *metricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errMetricsHijackNotSupported
	}
	return h.Hijack()
}

func (w *metricsResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
