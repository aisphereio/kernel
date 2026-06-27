package errorx

// Fields returns a structured, logger-neutral representation of err.
// logx/auditx/metricsx can convert it to concrete logger fields or labels.
func Fields(err error) map[string]any {
	fields := map[string]any{
		"error_code":  CodeOf(err).String(),
		"message":     MessageOf(err),
		"http_status": HTTPStatusOf(err),
		"grpc_code":   GRPCCodeOf(err),
		"retryable":   RetryableOf(err),
		"category":    CategoryOf(err).String(),
		"severity":    SeverityOf(err).String(),
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	if requestID := RequestIDOf(err); requestID != "" {
		fields["request_id"] = requestID
	}
	if traceID := TraceIDOf(err); traceID != "" {
		fields["trace_id"] = traceID
	}
	if md := SafeMetadataOf(err); len(md) > 0 {
		fields["error_metadata"] = md
	}
	if md := PublicMetadataOf(err); len(md) > 0 {
		fields["public_metadata"] = md
	}
	return fields
}

// MetricsLabels returns low-cardinality labels suitable for metrics. It never
// includes message, request_id, trace_id, metadata or dynamic object ids.
func MetricsLabels(err error) map[string]string {
	return map[string]string{
		"error_code":  CodeOf(err).String(),
		"http_status": intToString(HTTPStatusOf(err)),
		"grpc_code":   GRPCCodeOf(err),
		"retryable":   boolToString(RetryableOf(err)),
		"category":    CategoryOf(err).String(),
	}
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func intToString(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
