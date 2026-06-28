package casdoor

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

const (
	metricCasdoorRequestsTotal  = "kernel_authn_casdoor_requests_total"
	metricCasdoorRequestSeconds = "kernel_authn_casdoor_request_duration_seconds"
)

func casdoorLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("authn.casdoor").With(logx.String("provider", ProviderName))
}

func registerCasdoorMetrics(cfg Config) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	cfg.Metrics.NewCounter(metricCasdoorRequestsTotal, "Total Casdoor backend HTTP calls")
	cfg.Metrics.NewHistogram(metricCasdoorRequestSeconds, "Casdoor backend HTTP call latency in seconds", 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
}

func instrumentHTTPClient(base *http.Client, _ time.Duration, logger logx.Logger, metrics metricsx.Manager, metricsEnabled bool) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	clone := *base
	transport := clone.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = casdoorRoundTripper{next: transport, logger: logger, metrics: metrics, metricsEnabled: metricsEnabled}
	return &clone
}

type casdoorRoundTripper struct {
	next           http.RoundTripper
	logger         logx.Logger
	metrics        metricsx.Manager
	metricsEnabled bool
}

func (rt casdoorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.next == nil {
		rt.next = http.DefaultTransport
	}
	ctx := context.Background()
	if req != nil && req.Context() != nil {
		ctx = req.Context()
	}
	started := time.Now()
	resp, err := rt.next.RoundTrip(req)
	elapsed := time.Since(started)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	class := statusClass(status)
	method := ""
	path := ""
	if req != nil {
		method = req.Method
		path = safeURLPath(req.URL)
	}
	code := errorx.CodeOK.String()
	if err != nil {
		code = errorx.CodeUnavailable.String()
	} else if status >= 400 {
		code = codeFromHTTPStatus(status).String()
	}
	if rt.metricsEnabled && rt.metrics != nil {
		labels := []string{"method", method, "path", path, "status_class", class, "code", code}
		rt.metrics.IncrementCounter(ctx, metricCasdoorRequestsTotal, labels...)
		rt.metrics.RecordHistogram(ctx, metricCasdoorRequestSeconds, elapsed.Seconds(), labels...)
	}
	fields := []logx.Field{
		logx.String("method", method),
		logx.String("path", path),
		logx.Int("status", status),
		logx.String("status_class", class),
		logx.Duration("elapsed", elapsed),
	}
	if err != nil {
		rt.logger.Error("casdoor backend request failed", append(fields, logx.Err(errorx.Wrap(err, errorx.CodeUnavailable, errorx.WithMessage("casdoor backend request failed"))))...)
	} else if status >= 500 {
		rt.logger.Error("casdoor backend returned server error", fields...)
	} else if status >= 400 {
		rt.logger.Warn("casdoor backend returned client error", fields...)
	} else {
		rt.logger.Debug("casdoor backend request", fields...)
	}
	return resp, err
}

func safeURLPath(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

func statusClass(status int) string {
	if status <= 0 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}

func codeFromHTTPStatus(status int) errorx.Code {
	switch status {
	case http.StatusBadRequest:
		return errorx.CodeBadRequest
	case http.StatusUnauthorized:
		return errorx.CodeUnauthorized
	case http.StatusForbidden:
		return errorx.CodeForbidden
	case http.StatusNotFound:
		return errorx.CodeNotFound
	case http.StatusConflict:
		return errorx.CodeConflict
	case http.StatusTooManyRequests:
		return errorx.CodeTooManyRequests
	case http.StatusServiceUnavailable:
		return errorx.CodeUnavailable
	case http.StatusGatewayTimeout:
		return errorx.CodeTimeout
	default:
		if status >= 500 {
			return errorx.CodeUnavailable
		}
		if status >= 400 {
			return errorx.CodeBadRequest
		}
		return errorx.CodeOK
	}
}
