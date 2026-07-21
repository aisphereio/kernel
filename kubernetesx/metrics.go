package kubernetesx

import "github.com/aisphereio/kernel/logx"

// Metric names follow the kernel_<pkg>_* convention (see objectstorex). The
// observedClient decorator that records them is deferred to a follow-up PR;
// this file only registers the instruments so that the factory can advertise
// them and so that the decorator can record without re-registration.
const (
	metricKubernetesxOperationsTotal  = "kernel_kubernetesx_operations_total"
	metricKubernetesxOperationSeconds = "kernel_kubernetesx_operation_duration_seconds"
	metricKubernetesxProbeTotal       = "kernel_kubernetesx_probe_total"
	metricKubernetesxProbeSeconds     = "kernel_kubernetesx_probe_duration_seconds"

	// Default latency buckets for Kubernetes API calls, in seconds. Tuned
	// for the typical 5–500ms range of API-server operations with a long
	// tail for List/Apply.
	defaultBuckets = "0.005,0.01,0.025,0.05,0.1,0.25,0.5,1,2.5,5,10"
)

// defaultHistogramBuckets returns the latency buckets used by
// kubernetesx operation and probe histograms.
func defaultHistogramBuckets() []float64 {
	return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

// registerMetrics registers kubernetesx instruments on the supplied manager.
// It is nil-safe and guarded by MetricsEnabled, mirroring objectstorex.
func registerMetrics(cfg Config) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	m := cfg.Metrics
	m.NewCounter(metricKubernetesxOperationsTotal, "Total kubernetesx client operations")
	m.NewHistogram(metricKubernetesxOperationSeconds, "kubernetesx operation latency in seconds", defaultHistogramBuckets()...)
	m.NewCounter(metricKubernetesxProbeTotal, "Total kubernetesx probe operations")
	m.NewHistogram(metricKubernetesxProbeSeconds, "kubernetesx probe latency in seconds", defaultHistogramBuckets()...)
}

// kubernetesxLogger resolves the component logger, falling back to the
// default logger when Config.Logger is nil. Mirrors objectStoreLogger.
func kubernetesxLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("kubernetesx")
}
