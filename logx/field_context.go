package logx

import (
	"context"
	"log/slog"
)

type ctxFieldsKey struct{}
type ctxLoggerKey struct{}

func ContextWithFields(ctx context.Context, fields ...Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(fields) == 0 {
		return ctx
	}
	prev := FieldsFromContext(ctx)
	merged := make([]Field, 0, len(prev)+len(fields))
	merged = append(merged, prev...)
	merged = append(merged, fields...)
	return context.WithValue(ctx, ctxFieldsKey{}, merged)
}

func FieldsFromContext(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}
	fields, _ := ctx.Value(ctxFieldsKey{}).([]Field)
	return fields
}

// Inject stores request-scoped fields and a context-bound logger in ctx. It is
// the single primitive adapters should call after extracting request identity,
// trace identity, tenant/project info, etc.
func Inject(ctx context.Context, logger Logger, fields ...Field) context.Context {
	ctx = ContextWithFields(ctx, fields...)
	if logger == nil {
		logger = Noop()
	}
	return context.WithValue(ctx, ctxLoggerKey{}, logger.WithContext(ctx))
}

func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return Noop()
	}
	if logger, ok := ctx.Value(ctxLoggerKey{}).(Logger); ok && logger != nil {
		return logger
	}
	return Noop()
}

func FromContextOr(ctx context.Context, fallback Logger) Logger {
	if ctx == nil {
		if fallback == nil {
			return Noop()
		}
		return fallback
	}
	if logger, ok := ctx.Value(ctxLoggerKey{}).(Logger); ok && logger != nil {
		return logger
	}
	if fallback == nil {
		return Noop()
	}
	return fallback.WithContext(ctx)
}

func newFieldContextHandler(next slog.Handler, extractors ...FieldExtractor) slog.Handler {
	if next == nil {
		next = discardHandler{}
	}
	compact := extractors[:0]
	for _, fn := range extractors {
		if fn != nil {
			compact = append(compact, fn)
		}
	}
	if len(compact) == 0 {
		return next
	}
	return &fieldContextHandler{next: next, extractors: compact}
}

type fieldContextHandler struct {
	next       slog.Handler
	extractors []FieldExtractor
}

func (h *fieldContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *fieldContextHandler) Handle(ctx context.Context, record slog.Record) error {
	var fields []Field
	for _, fn := range h.extractors {
		fields = append(fields, fn(ctx)...)
	}
	if len(fields) > 0 {
		record = record.Clone()
		record.AddAttrs(fieldsToAttrs(fields, nil)...)
	}
	return h.next.Handle(ctx, record)
}

func (h *fieldContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fieldContextHandler{next: h.next.WithAttrs(attrs), extractors: h.extractors}
}

func (h *fieldContextHandler) WithGroup(name string) slog.Handler {
	return &fieldContextHandler{next: h.next.WithGroup(name), extractors: h.extractors}
}
