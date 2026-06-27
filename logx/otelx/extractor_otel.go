//go:build otel

package otelx

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/aisphereio/kernel/logx"
)

// TraceExtractor injects trace_id and span_id from an OpenTelemetry context.
// It is behind the `otel` build tag so logx core remains stdlib-only.
func TraceExtractor(ctx context.Context) []logx.Field {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return nil
	}
	sc := span.SpanContext()
	if !sc.IsValid() {
		return nil
	}
	return []logx.Field{
		logx.String("trace_id", sc.TraceID().String()),
		logx.String("span_id", sc.SpanID().String()),
	}
}
