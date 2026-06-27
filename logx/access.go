package logx

import "time"

type AccessEvent struct {
	Side       string
	Protocol   string
	Operation  string
	Component  string
	Method     string
	Path       string
	StatusCode int
	Code       string
	Reason     string
	Latency    time.Duration
	Err        error
	Fields     []Field
}

func LogAccess(logger Logger, e AccessEvent) {
	if logger == nil {
		logger = Noop()
	}
	fields := []Field{
		Event("access"),
		String("side", e.Side),
		String("protocol", e.Protocol),
		String("component", e.Component),
		String("operation", e.Operation),
		String("method", e.Method),
		String("path", e.Path),
		Int("status", e.StatusCode),
		String("code", e.Code),
		String("reason", e.Reason),
		Duration("latency", e.Latency),
		Int64("latency_ms", e.Latency.Milliseconds()),
	}
	if e.Err != nil {
		fields = append(fields, Err(e.Err))
	}
	fields = append(fields, e.Fields...)
	msg := "request completed"
	if e.Operation != "" {
		msg = e.Operation + " completed"
	}
	switch {
	case e.Err != nil || e.StatusCode >= 500:
		logger.Error(msg, fields...)
	case e.StatusCode >= 400:
		logger.Warn(msg, fields...)
	default:
		logger.Info(msg, fields...)
	}
}
