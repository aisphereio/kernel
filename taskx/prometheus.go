package taskx

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusObserver exports bounded-cardinality task execution metrics.
type PrometheusObserver struct {
	runs        *prometheus.CounterVec
	skips       *prometheus.CounterVec
	retries     *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	lastSuccess *prometheus.GaugeVec
}

// NewPrometheusObserver registers taskx metrics with reg. Create one observer
// per registry and share it across schedulers in the same process.
func NewPrometheusObserver(reg prometheus.Registerer) (*PrometheusObserver, error) {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	observer := &PrometheusObserver{
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kernel",
			Subsystem: "taskx",
			Name:      "runs_total",
			Help:      "Total number of terminal task runs by job and result.",
		}, []string{"job", "result"}),
		skips: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kernel",
			Subsystem: "taskx",
			Name:      "skips_total",
			Help:      "Total number of skipped task runs by job and reason.",
		}, []string{"job", "reason"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kernel",
			Subsystem: "taskx",
			Name:      "retries_total",
			Help:      "Total number of task retry attempts by job.",
		}, []string{"job"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "kernel",
			Subsystem: "taskx",
			Name:      "run_duration_seconds",
			Help:      "Duration of terminal task runs by job and result.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"job", "result"}),
		lastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "kernel",
			Subsystem: "taskx",
			Name:      "last_success_unixtime",
			Help:      "Unix timestamp of the last successful run by job.",
		}, []string{"job"}),
	}
	collectors := []prometheus.Collector{
		observer.runs,
		observer.skips,
		observer.retries,
		observer.duration,
		observer.lastSuccess,
	}
	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			return nil, err
		}
	}
	return observer, nil
}

func (o *PrometheusObserver) Observe(_ context.Context, event Event) {
	if o == nil {
		return
	}
	switch event.Kind {
	case EventSucceeded:
		o.runs.WithLabelValues(event.JobName, "success").Inc()
		o.duration.WithLabelValues(event.JobName, "success").Observe(event.Duration.Seconds())
		o.lastSuccess.WithLabelValues(event.JobName).Set(float64(event.Timestamp.Unix()))
	case EventFailed:
		o.runs.WithLabelValues(event.JobName, "failure").Inc()
		o.duration.WithLabelValues(event.JobName, "failure").Observe(event.Duration.Seconds())
	case EventSkipped:
		o.skips.WithLabelValues(event.JobName, event.Reason).Inc()
	case EventRetrying:
		o.retries.WithLabelValues(event.JobName).Inc()
	}
}
