package logx

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
)

const defaultRedactedValue = "***"

type Redactor interface {
	Redact(Field) Field
}

type keyRedactor struct {
	enabled bool
	keys    map[string]struct{}
	value   string
}

func NewRedactor(cfg RedactConfig) Redactor {
	keys := cfg.Keys
	if len(keys) == 0 {
		keys = DefaultRedactKeys()
	}
	value := cfg.Value
	if value == "" {
		value = defaultRedactedValue
	}
	m := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		m[normalizeSensitiveKey(key)] = struct{}{}
	}
	return &keyRedactor{enabled: cfg.Enabled, keys: m, value: value}
}

func DefaultRedactKeys() []string {
	return []string{
		"password", "passwd", "pwd", "token", "access_token", "refresh_token", "id_token",
		"secret", "client_secret", "authorization", "cookie", "set_cookie",
		"api_key", "private_key", "ak", "sk",
	}
}

func (r *keyRedactor) Redact(field Field) Field {
	if r == nil || !r.enabled || field.Key == "" {
		return field
	}
	if _, ok := r.keys[normalizeSensitiveKey(field.Key)]; ok {
		return Field{Key: field.Key, Value: r.value}
	}
	return field
}

func normalizeSensitiveKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, ".", "_")
	return key
}

func newRedactHandler(next slog.Handler, redactor Redactor) slog.Handler {
	if redactor == nil {
		return next
	}
	return &redactHandler{next: next, redactor: redactor}
}

type redactHandler struct {
	next     slog.Handler
	redactor Redactor
}

func (h *redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.redactor == nil || record.NumAttrs() == 0 {
		return h.next.Handle(ctx, record)
	}
	clone := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clone.AddAttrs(redactAttr(attr, h.redactor))
		return true
	})
	return h.next.Handle(ctx, clone)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &redactHandler{next: h.next.WithAttrs(redactAttrs(attrs, h.redactor)), redactor: h.redactor}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{next: h.next.WithGroup(name), redactor: h.redactor}
}

func redactAttrs(attrs []slog.Attr, redactor Redactor) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, redactAttr(attr, redactor))
	}
	return out
}

func redactAttr(attr slog.Attr, redactor Redactor) slog.Attr {
	if redactor == nil || attr.Key == "" {
		return attr
	}
	if attr.Value.Kind() == slog.KindGroup {
		return slog.Attr{Key: attr.Key, Value: slog.GroupValue(redactAttrs(attr.Value.Group(), redactor)...)}
	}
	field := redactor.Redact(Field{Key: attr.Key, Value: attr.Value.Any()})
	if field.Key != attr.Key || !reflect.DeepEqual(field.Value, attr.Value.Any()) {
		return slog.Any(field.Key, field.Value)
	}
	return attr
}
