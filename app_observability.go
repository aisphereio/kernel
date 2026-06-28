package kernel

import (
	"context"

	"github.com/aisphereio/kernel/logx"
)

func configureDefaultLogger(o *options) logx.Logger {
	if o == nil {
		return logx.DefaultLogger()
	}
	if o.logxLogger != nil {
		if slogLogger, err := logx.Slog(o.logxLogger); err == nil && slogLogger != nil {
			logx.SetDefault(slogLogger)
		}
		return o.logxLogger
	}
	if o.logger != nil {
		logx.SetDefault(o.logger)
		return logx.FromSlog(o.logger)
	}
	return logx.DefaultLogger()
}

func (a *App) logger() logx.Logger {
	if a == nil {
		return logx.DefaultLogger()
	}
	logger := a.opts.logxLogger
	if logger == nil && a.opts.logger != nil {
		logger = logx.FromSlog(a.opts.logger)
	}
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("kernel.app").With(appLogFields(a.opts)...)
}

func appLogFields(o options) []logx.Field {
	fields := []logx.Field{
		logx.String("app_id", o.id),
		logx.String("service", o.name),
		logx.String("version", o.version),
	}
	for k, v := range o.metadata {
		fields = append(fields, logx.String("metadata."+k, v))
	}
	return fields
}

func injectAppLogger(ctx context.Context, logger logx.Logger, fields ...logx.Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logx.Inject(ctx, logger, fields...)
}
