package kernel

import (
	"context"

	"github.com/aisphereio/kernel/dtmx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

func configureDefaultMetrics(o *options, logger logx.Logger) metricsx.Manager {
	if o == nil {
		return metricsx.Noop()
	}
	manager := o.metrics
	if manager == nil && o.prometheusMetrics {
		appName := o.name
		if appName == "" {
			appName = "kernel"
		}
		appVersion := o.version
		if appVersion == "" {
			appVersion = "dev"
		}
		manager = metricsx.NewPrometheusManager(appName, appVersion, logger)
	}
	manager = metricsx.Ensure(manager)
	if o.metricsSystem {
		metricsx.RegisterSystemMetrics(manager)
	}
	return manager
}

func (a *App) metrics() metricsx.Manager {
	if a == nil {
		return metricsx.Noop()
	}
	return metricsx.Ensure(a.opts.metrics)
}

// MetricsFromContext returns the metrics manager injected by kernel.App, or a
// no-op manager when metrics are disabled. This keeps lifecycle hooks and
// component setup code independent from the concrete metrics package bootstrap.
func MetricsFromContext(ctx context.Context) metricsx.Manager {
	return metricsx.FromContext(ctx)
}

func injectAppObservability(ctx context.Context, logger logx.Logger, manager metricsx.Manager, dtm dtmx.Manager, fields ...logx.Field) context.Context {
	ctx = injectAppLogger(ctx, logger, fields...)
	ctx = metricsx.Inject(ctx, manager)
	return dtmx.Inject(ctx, dtm)
}
