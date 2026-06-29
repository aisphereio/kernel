// Package restx contains go-zero-inspired HTTP runtime helpers for Kernel.
//
// restx deliberately does not replace transportx/http. It builds a standard
// HTTP filter chain and server options on top of the existing transport so that
// Kernel services get a consistent startup path: recover, max bytes, gunzip,
// CORS/security headers, max-connections, breaker and adaptive shedding.
package restx

import (
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/servicecontextx"
	httpx "github.com/aisphereio/kernel/transportx/http"
)

// Conf is the high-level REST server configuration used by new Kernel services.
type Conf struct {
	Network string
	Address string
	Timeout time.Duration

	// ServiceName is used by tracing, metrics and access logs.
	ServiceName string
	// Logger is injected into request contexts and used by the standard log filter.
	Logger logx.Logger
	// Metrics is injected into request contexts and used by the standard metrics filter.
	Metrics metricsx.Manager
	// ServiceContext is optionally injected into request contexts for generated
	// Gateway/BFF logic that follows the go-zero ServiceContext pattern.
	ServiceContext *servicecontextx.Context

	Middlewares MiddlewaresConf
}

// MiddlewaresConf mirrors the useful part of go-zero's RestConf.Middlewares,
// adapted to Kernel's transportx/http FilterFunc chain.
type MiddlewaresConf struct {
	Trace           bool // reserved for tracingx/otel integration
	Log             bool // transportx/http already owns access logs; kept for config compatibility
	Prometheus      bool // transport metrics are enabled via transportx/http.Metrics
	MaxConns        bool
	Breaker         bool
	Shedding        bool
	Timeout         bool
	Recover         bool
	Metrics         bool // reserved for app-level metrics filters
	MaxBytes        bool
	Gunzip          bool
	CORS            bool
	CSRF            bool
	SecurityHeaders bool
	SanitizeHeaders bool

	MaxConnsLimit int
	MaxBytesLimit int64
	TimeoutValue  time.Duration
	CORSConfig    CORSConfig
	CSRFConfig    CSRFConfig
}

// DefaultMiddlewares returns a conservative production-friendly chain. It keeps
// authn/authz out of the generic chain; services add those explicitly.
func DefaultMiddlewares() MiddlewaresConf {
	return MiddlewaresConf{
		Trace:           true,
		Log:             true,
		Metrics:         true,
		MaxConns:        true,
		Breaker:         true,
		Shedding:        true,
		Timeout:         true,
		Recover:         true,
		MaxBytes:        true,
		Gunzip:          true,
		SecurityHeaders: true,
		SanitizeHeaders: true,
		MaxConnsLimit:   1024,
		MaxBytesLimit:   32 << 20,
		TimeoutValue:    3 * time.Second,
	}
}

// Filters builds the Kernel HTTP filter chain in go-zero order where the
// concepts overlap.
func Filters(conf MiddlewaresConf) []httpx.FilterFunc {
	return RuntimeFilters(Runtime{Middlewares: conf})
}

// Runtime carries non-serializable runtime dependencies for the standard chain.
type Runtime struct {
	ServiceName    string
	Logger         logx.Logger
	Metrics        metricsx.Manager
	ServiceContext *servicecontextx.Context
	Middlewares    MiddlewaresConf
}

// RuntimeFilters builds the Kernel HTTP filter chain in go-zero order and also
// injects request-scoped contextx/logx/metricsx values when configured.
func RuntimeFilters(rt Runtime) []httpx.FilterFunc {
	conf := rt.Middlewares
	filters := make([]httpx.FilterFunc, 0, 14)
	serviceName := rt.ServiceName
	if serviceName == "" {
		serviceName = "kernel"
	}
	if conf.Trace {
		filters = append(filters, Trace(serviceName))
	}
	filters = append(filters, RequestContext(RequestContextConfig{
		Logger:         rt.Logger,
		Metrics:        rt.Metrics,
		ServiceContext: rt.ServiceContext,
	}))
	if conf.Log {
		filters = append(filters, AccessLog(rt.Logger))
	}
	if conf.Metrics {
		filters = append(filters, Metrics(rt.Metrics))
	}
	if conf.SanitizeHeaders {
		filters = append(filters, SanitizeHeaders())
	}
	if conf.SecurityHeaders {
		filters = append(filters, SecurityHeaders())
	}
	if conf.CORS {
		filters = append(filters, CORS(conf.CORSConfig))
	}
	if conf.MaxConns {
		limit := conf.MaxConnsLimit
		if limit <= 0 {
			limit = 1024
		}
		filters = append(filters, MaxConns(limit))
	}
	if conf.Breaker {
		filters = append(filters, Breaker())
	}
	if conf.Shedding {
		filters = append(filters, Shedding())
	}
	if conf.Timeout {
		timeout := conf.TimeoutValue
		if timeout <= 0 {
			timeout = 3 * time.Second
		}
		filters = append(filters, Timeout(timeout))
	}
	if conf.Recover {
		filters = append(filters, Recover())
	}
	if conf.MaxBytes {
		limit := conf.MaxBytesLimit
		if limit <= 0 {
			limit = 32 << 20
		}
		filters = append(filters, MaxBytes(limit))
	}
	if conf.Gunzip {
		filters = append(filters, Gunzip())
	}
	if conf.CSRF {
		filters = append(filters, CSRF(conf.CSRFConfig))
	}
	return filters
}

// ServerOptions converts Conf to existing transportx/http options. Extra
// filters are appended after the standard chain and before the router.
func ServerOptions(conf Conf, extra ...httpx.FilterFunc) []httpx.ServerOption {
	opts := make([]httpx.ServerOption, 0, 4)
	if conf.Network != "" {
		opts = append(opts, httpx.Network(conf.Network))
	}
	if conf.Address != "" {
		opts = append(opts, httpx.Address(conf.Address))
	}
	if conf.Timeout > 0 {
		opts = append(opts, httpx.Timeout(conf.Timeout))
	}
	chain := append(RuntimeFilters(Runtime{ServiceName: conf.ServiceName, Logger: conf.Logger, Metrics: conf.Metrics, ServiceContext: conf.ServiceContext, Middlewares: conf.Middlewares}), extra...)
	if len(chain) > 0 {
		opts = append(opts, httpx.Filter(chain...))
	}
	return opts
}
