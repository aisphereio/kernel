package logx

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger is the business-facing Kernel logger. It does not expose slog.Logger
// or slog.Attr to application packages.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	Named(name string) Logger
	WithContext(ctx context.Context) Logger
	Enabled(level LogLevel) bool
	Sync() error
}

// FromSlog adapts an existing slog logger to the business-facing logx.Logger
// interface. It is useful for Kernel adapters that need logx semantics while
// still honoring slog.Default or an application-provided slog logger.
func FromSlog(logger *slog.Logger) Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &slogLogger{l: logger, ctx: context.Background()}
}

// DefaultLogger returns slog.Default adapted as a logx.Logger.
func DefaultLogger() Logger { return FromSlog(slog.Default()) }

type slogLogger struct {
	l        *slog.Logger
	ctx      context.Context
	redactor Redactor
	closer   io.Closer
}

// New creates the default slog-backed logx logger.
func New(cfg Config, opts ...Option) (Logger, LevelController, error) {
	return NewSlog(cfg, opts...)
}

// NewSlog creates a slog-backed logx logger while exposing only the logx.Logger
// interface to business code.
func NewSlog(cfg Config, opts ...Option) (Logger, LevelController, error) {
	level, err := ParseLogLevel(cfg.Level)
	if err != nil {
		return nil, nil, err
	}
	levelVar := &slog.LevelVar{}
	levelVar.Set(level.slogLevel())

	builder := &handlerConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(builder)
		}
	}

	writer, closer, err := openWriter(cfg.Output, builder.writer)
	if err != nil {
		return nil, nil, err
	}

	redactor := NewRedactor(cfg.Redact)
	handlerOpts := &slog.HandlerOptions{
		Level:       levelVar,
		AddSource:   cfg.AddSource,
		ReplaceAttr: composeReplaceAttr(builder.replaceAttr),
	}
	var h slog.Handler
	switch normalizeFormat(cfg.Format) {
	case FormatJSON:
		h = slog.NewJSONHandler(writer, handlerOpts)
	default:
		h = slog.NewTextHandler(writer, handlerOpts)
	}

	h = newRedactHandler(h, redactor)
	if cfg.Sampling.Enabled {
		h = newSamplingHandler(h, cfg.Sampling)
	}
	if len(builder.dropFilters) > 0 {
		h = newDropFilterHandler(h, builder.dropFilters...)
	}
	h = newFieldContextHandler(h, append([]FieldExtractor{FieldsFromContext}, builder.fieldExtractors...)...)

	base := slog.New(h).With(
		slog.String("service", cfg.ServiceName),
		slog.String("env", cfg.Env),
		slog.String("version", cfg.Version),
	)
	if cfg.NodeID != "" {
		base = base.With(slog.String("node_id", cfg.NodeID))
	}

	return &slogLogger{l: base, redactor: redactor, closer: closer}, slogLevelController{level: levelVar}, nil
}

func (l *slogLogger) Debug(msg string, fields ...Field) { l.log(DebugLevel, msg, fields...) }
func (l *slogLogger) Info(msg string, fields ...Field)  { l.log(InfoLevel, msg, fields...) }
func (l *slogLogger) Warn(msg string, fields ...Field)  { l.log(WarnLevel, msg, fields...) }
func (l *slogLogger) Error(msg string, fields ...Field) { l.log(ErrorLevel, msg, fields...) }

func (l *slogLogger) log(level LogLevel, msg string, fields ...Field) {
	if l == nil || l.l == nil {
		return
	}
	ctx := l.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	slogLevel := level.slogLevel()
	if !l.l.Enabled(ctx, slogLevel) {
		return
	}
	pc := sourcePC(4)
	record := slog.NewRecord(time.Now(), slogLevel, msg, pc)
	record.AddAttrs(fieldsToAttrs(fields, l.redactor)...)
	_ = l.l.Handler().Handle(ctx, record)
}

func (l *slogLogger) With(fields ...Field) Logger {
	if l == nil || l.l == nil {
		return Noop()
	}
	attrs := fieldsToAttrs(fields, l.redactor)
	return &slogLogger{l: l.l.With(attrsToAny(attrs)...), ctx: l.ctx, redactor: l.redactor, closer: l.closer}
}

func (l *slogLogger) Named(name string) Logger {
	if l == nil || l.l == nil || name == "" {
		return l
	}
	return l.With(String("module", name))
}

func (l *slogLogger) WithContext(ctx context.Context) Logger {
	if l == nil || l.l == nil {
		return Noop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &slogLogger{l: l.l, ctx: ctx, redactor: l.redactor, closer: l.closer}
}

func (l *slogLogger) Enabled(level LogLevel) bool {
	if l == nil || l.l == nil {
		return false
	}
	ctx := l.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return l.l.Enabled(ctx, level.slogLevel())
}

func (l *slogLogger) Sync() error {
	if l == nil || l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

func Sync(logger Logger) error {
	if logger == nil {
		return nil
	}
	return logger.Sync()
}

func openWriter(output string, injected io.Writer) (io.Writer, io.Closer, error) {
	if injected != nil {
		return injected, nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "", string(OutputStdout):
		return os.Stdout, nil, nil
	case string(OutputStderr):
		return os.Stderr, nil, nil
	default:
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return nil, nil, err
		}
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}
		return f, f, nil
	}
}

func normalizeFormat(format Format) Format {
	switch strings.ToLower(strings.TrimSpace(string(format))) {
	case "", string(FormatJSON):
		return FormatJSON
	case string(FormatConsole), "text":
		return FormatConsole
	default:
		return FormatJSON
	}
}

func composeReplaceAttr(user func(groups []string, a slog.Attr) slog.Attr) func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
			a.Key = "timestamp"
		}
		if a.Key == slog.MessageKey {
			a.Key = "message"
		}
		if a.Key == slog.LevelKey {
			a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
		}
		if user != nil {
			a = user(groups, a)
		}
		return a
	}
}

var errUnsupportedLogger = errors.New("unsupported logger implementation")

// Slog returns the underlying slog logger only for framework adapters.
// Business packages should not call this helper.
func Slog(logger Logger) (*slog.Logger, error) {
	if l, ok := logger.(*slogLogger); ok && l != nil {
		return l.l, nil
	}
	return nil, errUnsupportedLogger
}
