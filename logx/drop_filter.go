package logx

import (
	"context"
	"log/slog"
)

// Entry is a normalized log record used by filters and test loggers.
type Entry struct {
	Level   LogLevel
	Message string
	Fields  []Field
}

func newDropFilterHandler(next slog.Handler, filters ...DropFilter) slog.Handler {
	compact := make([]DropFilter, 0, len(filters))
	for _, filter := range filters {
		if filter != nil {
			compact = append(compact, filter)
		}
	}
	if len(compact) == 0 {
		return next
	}
	return &dropFilterHandler{next: next, filters: compact}
}

type dropFilterHandler struct {
	next    slog.Handler
	filters []DropFilter
}

func (h *dropFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *dropFilterHandler) Handle(ctx context.Context, record slog.Record) error {
	entry := entryFromRecord(record)
	for _, filter := range h.filters {
		if filter(ctx, entry) {
			return nil
		}
	}
	return h.next.Handle(ctx, record)
}

func (h *dropFilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &dropFilterHandler{next: h.next.WithAttrs(attrs), filters: h.filters}
}

func (h *dropFilterHandler) WithGroup(name string) slog.Handler {
	return &dropFilterHandler{next: h.next.WithGroup(name), filters: h.filters}
}

func entryFromRecord(record slog.Record) Entry {
	fields := make([]Field, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, Field{Key: attr.Key, Value: attr.Value.Any()})
		return true
	})
	return Entry{Level: logLevelFromSlog(record.Level), Message: record.Message, Fields: fields}
}

func DropEvents(events ...string) DropFilter {
	set := make(map[string]struct{}, len(events))
	for _, event := range events {
		if event != "" {
			set[event] = struct{}{}
		}
	}
	return func(_ context.Context, e Entry) bool {
		for _, field := range e.Fields {
			if field.Key == "event" {
				_, ok := set[stringify(field.Value)]
				return ok
			}
		}
		return false
	}
}

func DropMessages(messages ...string) DropFilter {
	set := make(map[string]struct{}, len(messages))
	for _, msg := range messages {
		if msg != "" {
			set[msg] = struct{}{}
		}
	}
	return func(_ context.Context, e Entry) bool {
		_, ok := set[e.Message]
		return ok
	}
}
