package logx

import "time"

type ExternalCall struct {
	Provider       string
	Service        string
	Operation      string
	Model          string
	Endpoint       string
	StatusCode     int
	UpstreamStatus string
	Latency        time.Duration
	Err            error
	Fields         []Field
}

func LogExternalCall(logger Logger, call ExternalCall) {
	if logger == nil {
		logger = Noop()
	}
	fields := []Field{
		Event("external_call"),
		String("provider", call.Provider),
		String("upstream_service", call.Service),
		String("operation", call.Operation),
		String("model", call.Model),
		String("endpoint", call.Endpoint),
		Int("status", call.StatusCode),
		String("upstream_status", call.UpstreamStatus),
		Duration("latency", call.Latency),
		Int64("latency_ms", call.Latency.Milliseconds()),
	}
	if call.Err != nil {
		fields = append(fields, Err(call.Err))
	}
	fields = append(fields, call.Fields...)
	if call.Err != nil || call.StatusCode >= 500 {
		logger.Error("external call failed", fields...)
		return
	}
	if call.StatusCode >= 400 {
		logger.Warn("external call completed with client error", fields...)
		return
	}
	logger.Info("external call completed", fields...)
}
