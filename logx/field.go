package logx

import (
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"time"
)

// Field is the only field type exposed to business code.
// Do not alias slog.Attr here, otherwise the Kernel ABI leaks the engine.
type Field struct {
	Key   string
	Value any
}

func String(key, value string) Field                 { return Field{Key: key, Value: value} }
func Int(key string, value int) Field                { return Field{Key: key, Value: value} }
func Int64(key string, value int64) Field            { return Field{Key: key, Value: value} }
func Uint64(key string, value uint64) Field          { return Field{Key: key, Value: value} }
func Bool(key string, value bool) Field              { return Field{Key: key, Value: value} }
func Float64(key string, value float64) Field        { return Field{Key: key, Value: value} }
func Duration(key string, value time.Duration) Field { return Field{Key: key, Value: value} }
func Time(key string, value time.Time) Field         { return Field{Key: key, Value: value} }
func Any(key string, value any) Field                { return Field{Key: key, Value: value} }
func Event(name string) Field                        { return String("event", name) }
func Err(err error) Field                            { return Field{Key: "error", Value: err} }

// Group is useful when a value is naturally nested, but most platform fields
// should stay flat for Loki/ELK/ClickHouse indexing.
func Group(key string, fields ...Field) Field { return Field{Key: key, Value: fields} }

// Redacter can be implemented by request/response objects that explicitly know
// how to produce a safe log representation. Unlike Kratos, logx does not print
// arbitrary request objects by default.
type Redacter interface {
	Redact() any
}

func fieldsToAttrs(fields []Field, redactor Redactor) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields)+4)
	for _, field := range fields {
		attrs = appendField(attrs, field, redactor)
	}
	return attrs
}

func appendField(attrs []slog.Attr, field Field, redactor Redactor) []slog.Attr {
	if field.Key == "" {
		return attrs
	}
	if redactor != nil {
		field = redactor.Redact(field)
	}
	if field.Value == nil {
		return append(attrs, slog.Any(field.Key, nil))
	}
	if field.Key == "error" {
		if err, ok := field.Value.(error); ok {
			return appendErrorAttrs(attrs, err)
		}
	}
	if nested, ok := field.Value.([]Field); ok {
		groupAttrs := fieldsToAttrs(nested, redactor)
		return append(attrs, slog.Group(field.Key, attrsToAny(groupAttrs)...))
	}
	switch v := field.Value.(type) {
	case string:
		return append(attrs, slog.String(field.Key, v))
	case int:
		return append(attrs, slog.Int(field.Key, v))
	case int64:
		return append(attrs, slog.Int64(field.Key, v))
	case uint64:
		return append(attrs, slog.Uint64(field.Key, v))
	case bool:
		return append(attrs, slog.Bool(field.Key, v))
	case float64:
		return append(attrs, slog.Float64(field.Key, v))
	case time.Duration:
		return append(attrs, slog.Duration(field.Key, v), slog.Int64(field.Key+"_ms", v.Milliseconds()))
	case time.Time:
		return append(attrs, slog.Time(field.Key, v))
	case error:
		return append(attrs, slog.String(field.Key, v.Error()))
	case Redacter:
		return append(attrs, slog.Any(field.Key, v.Redact()))
	default:
		return append(attrs, slog.Any(field.Key, v))
	}
}

func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, a)
	}
	return out
}

func appendErrorAttrs(attrs []slog.Attr, err error) []slog.Attr {
	if err == nil {
		return attrs
	}
	attrs = append(attrs,
		slog.String("error", err.Error()),
		slog.String("error_type", typeName(err)),
	)
	if e, ok := err.(codeCarrier); ok && e.Code() != "" {
		attrs = append(attrs, slog.String("error_code", e.Code()))
	} else if e, ok := err.(errorCodeCarrier); ok && e.ErrorCode() != "" {
		attrs = append(attrs, slog.String("error_code", e.ErrorCode()))
	}
	if e, ok := err.(reasonCarrier); ok && e.Reason() != "" {
		attrs = append(attrs, slog.String("error_reason", e.Reason()))
	}
	if e, ok := err.(httpStatusCarrier); ok && e.HTTPStatus() > 0 {
		attrs = append(attrs, slog.Int("http_status", e.HTTPStatus()))
	} else if e, ok := err.(statusCodeCarrier); ok && e.StatusCode() > 0 {
		attrs = append(attrs, slog.Int("http_status", e.StatusCode()))
	}
	if e, ok := err.(retryableCarrier); ok {
		attrs = append(attrs, slog.Bool("retryable", e.Retryable()))
	}
	if e, ok := err.(grpcCodeStringCarrier); ok && e.GRPCCode() != "" {
		attrs = append(attrs, slog.String("grpc_code", e.GRPCCode()))
	} else if e, ok := err.(grpcCodeIntCarrier); ok {
		attrs = append(attrs, slog.Int("grpc_code", e.GRPCCode()))
	}
	return attrs
}

func typeName(v any) string {
	if v == nil {
		return ""
	}
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	return t.String()
}

func sourcePC(skip int) uintptr {
	pcs := make([]uintptr, 1)
	if runtime.Callers(skip, pcs) == 0 {
		return 0
	}
	return pcs[0]
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprint(v)
}
