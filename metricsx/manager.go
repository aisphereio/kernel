package metricsx

import (
	"context"
	"fmt"
	"sync"

	"github.com/aisphereio/kernel/logx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ============================================================================
// Logger interface
// ============================================================================

// Logger is the Kernel logger interface accepted by metricsx.
type Logger = logx.Logger

// noopLogger discards all log output.
type noopLogger struct{ logx.Logger }

// ============================================================================
// Manager interface
// ============================================================================

// Manager defines the interface for registering and recording metrics.
// All methods are nil-safe — a nil Manager is a valid no-op implementation.
type Manager interface {
	// Registration (call at startup, before recording)
	NewCounter(name, desc string)
	NewUpDownCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)
	NewGauge(name, desc string)

	// Recording (call in business code)
	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}

// ============================================================================
// otelManager (default implementation backed by otel/metric)
// ============================================================================

// otelManager implements Manager backed by go.opentelemetry.io/otel/metric.
type otelManager struct {
	meter  metric.Meter
	store  *store
	logger Logger
}

// NewManager creates a Manager backed by the given otel meter.
// If logger is nil, a no-op logger is used.
func NewManager(meter metric.Meter, logger Logger) Manager {
	if logger == nil {
		logger = logx.Noop()
	}
	if meter == nil {
		// Return nil Manager (no-op) if meter is nil — see nilManager below.
		return (*nilManager)(nil)
	}
	return &otelManager{
		meter:  meter,
		store:  newStore(),
		logger: logger,
	}
}

// NewCounter registers a monotonic int64 counter.
// Errors are logged via the Manager's logger; the method does not return error
// to keep call sites clean.
func (m *otelManager) NewCounter(name, desc string) {
	counter, err := m.meter.Int64Counter(name, metric.WithDescription(desc))
	if err != nil {
		m.logger.Error("metricsx register counter failed", logx.String("metric", name), logx.Err(err))
		return
	}
	if err := m.store.setCounter(name, counter); err != nil {
		m.logger.Error("metricsx register counter failed", logx.String("metric", name), logx.Err(err))
	}
}

// NewUpDownCounter registers a float64 up-down counter (can increase or decrease).
func (m *otelManager) NewUpDownCounter(name, desc string) {
	udc, err := m.meter.Float64UpDownCounter(name, metric.WithDescription(desc))
	if err != nil {
		m.logger.Error("metricsx register up-down counter failed", logx.String("metric", name), logx.Err(err))
		return
	}
	if err := m.store.setUpDownCounter(name, udc); err != nil {
		m.logger.Error("metricsx register up-down counter failed", logx.String("metric", name), logx.Err(err))
	}
}

// NewHistogram registers a float64 histogram with the given bucket boundaries.
// If no buckets are provided, the SDK default is used.
//
//	m.NewHistogram("latency_seconds", "Request latency", 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5)
func (m *otelManager) NewHistogram(name, desc string, buckets ...float64) {
	opts := []metric.Float64HistogramOption{metric.WithDescription(desc)}
	if len(buckets) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(buckets...))
	}
	hist, err := m.meter.Float64Histogram(name, opts...)
	if err != nil {
		m.logger.Error("metricsx register histogram failed", logx.String("metric", name), logx.Err(err))
		return
	}
	if err := m.store.setHistogram(name, hist); err != nil {
		m.logger.Error("metricsx register histogram failed", logx.String("metric", name), logx.Err(err))
	}
}

// NewGauge registers a float64 gauge (current value, observable asynchronously).
func (m *otelManager) NewGauge(name, desc string) {
	g := &float64Gauge{observations: make(map[attribute.Set]float64)}
	_, err := m.meter.Float64ObservableGauge(
		name,
		metric.WithDescription(desc),
		metric.WithFloat64Callback(g.callback),
	)
	if err != nil {
		m.logger.Error("metricsx register gauge failed", logx.String("metric", name), logx.Err(err))
		return
	}
	if err := m.store.setGauge(name, g); err != nil {
		m.logger.Error("metricsx register gauge failed", logx.String("metric", name), logx.Err(err))
	}
}

// IncrementCounter adds 1 to the named counter.
// If name is not registered, logs an error and no-ops.
func (m *otelManager) IncrementCounter(ctx context.Context, name string, labels ...string) {
	counter, err := m.store.getCounter(name)
	if err != nil {
		m.logger.Error("metricsx counter not registered", logx.String("metric", name), logx.Err(err))
		return
	}
	counter.Add(ctx, 1, metric.WithAttributes(m.attrs(name, labels...)...))
}

// DeltaUpDownCounter adds value (can be negative) to the named up-down counter.
func (m *otelManager) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	udc, err := m.store.getUpDownCounter(name)
	if err != nil {
		m.logger.Error("metricsx up-down counter not registered", logx.String("metric", name), logx.Err(err))
		return
	}
	udc.Add(ctx, value, metric.WithAttributes(m.attrs(name, labels...)...))
}

// RecordHistogram records value in the named histogram.
func (m *otelManager) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	hist, err := m.store.getHistogram(name)
	if err != nil {
		m.logger.Error("metricsx histogram not registered", logx.String("metric", name), logx.Err(err))
		return
	}
	hist.Record(ctx, value, metric.WithAttributes(m.attrs(name, labels...)...))
}

// SetGauge sets the named gauge to value.
func (m *otelManager) SetGauge(name string, value float64, labels ...string) {
	g, err := m.store.getGauge(name)
	if err != nil {
		m.logger.Error("metricsx gauge not registered", logx.String("metric", name), logx.Err(err))
		return
	}
	g.set(value, attribute.NewSet(m.attrs(name, labels...)...))
}

// attrs converts string key-value pairs to otel attributes, with cardinality
// validation.
func (m *otelManager) attrs(name string, labels ...string) []attribute.KeyValue {
	count := len(labels)
	if count%2 != 0 {
		m.logger.Warn("metricsx odd number of labels, last label ignored", logx.String("metric", name), logx.Int("label_count", count))
	}
	const cardinalityLimit = 20
	if count > cardinalityLimit {
		m.logger.Warn("metricsx high label cardinality", logx.String("metric", name), logx.Int("label_count", count), logx.Int("limit", cardinalityLimit))
	}
	out := make([]attribute.KeyValue, 0, count/2)
	for i := 0; i+1 < count; i += 2 {
		out = append(out, attribute.String(labels[i], labels[i+1]))
	}
	return out
}

// ============================================================================
// nilManager (no-op implementation for nil safety)
// ============================================================================

// nilManager is a Manager whose receiver is always nil. All methods are no-ops.
// This is returned by NewManager when meter is nil, so business code can pass
// a nil Manager to tests without nil-checks.
type nilManager struct{}

func (*nilManager) NewCounter(string, string)                                      {}
func (*nilManager) NewUpDownCounter(string, string)                                {}
func (*nilManager) NewHistogram(string, string, ...float64)                        {}
func (*nilManager) NewGauge(string, string)                                        {}
func (*nilManager) IncrementCounter(context.Context, string, ...string)            {}
func (*nilManager) DeltaUpDownCounter(context.Context, string, float64, ...string) {}
func (*nilManager) RecordHistogram(context.Context, string, float64, ...string)    {}
func (*nilManager) SetGauge(string, float64, ...string)                            {}

// Noop returns a Manager that discards all operations. Useful in tests and
// disabled features.
func Noop() Manager { return (*nilManager)(nil) }

// ============================================================================
// float64Gauge (synchronous gauge — otel only provides async observable gauge)
// ============================================================================

type float64Gauge struct {
	mu           sync.RWMutex
	observations map[attribute.Set]float64
}

func (g *float64Gauge) callback(_ context.Context, o metric.Float64Observer) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for attrs, val := range g.observations {
		o.Observe(val, metric.WithAttributeSet(attrs))
	}
	return nil
}

func (g *float64Gauge) set(val float64, attrs attribute.Set) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.observations[attrs] = val
}

// ============================================================================
// Errors
// ============================================================================

// ErrAlreadyRegistered is returned when a metric name is registered twice.
type ErrAlreadyRegistered struct{ Name string }

func (e *ErrAlreadyRegistered) Error() string {
	return fmt.Sprintf("metricsx: metric %q already registered", e.Name)
}

// ErrNotRegistered is returned when recording against an unregistered name.
type ErrNotRegistered struct{ Name string }

func (e *ErrNotRegistered) Error() string {
	return fmt.Sprintf("metricsx: metric %q not registered", e.Name)
}
