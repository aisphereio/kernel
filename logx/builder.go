package logx

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Format selects the encoding used by the default handler builder.
type Format string

const (
	// FormatText writes records using [slog.NewTextHandler].
	FormatText Format = "console"
	// FormatConsole is an alias of FormatText for config files.
	FormatConsole Format = FormatText
	// FormatJSON writes records using [slog.NewJSONHandler].
	FormatJSON Format = "json"
)

// Option configures [NewHandler] and the decorators applied by [NewLogger].
type Option func(*handlerConfig)

// Extractor extracts attrs from a log call context.
type Extractor func(context.Context) []slog.Attr

type handlerConfig struct {
	writer          io.Writer
	format          Format
	level           Leveler
	addSource       bool
	replaceAttr     func(groups []string, a slog.Attr) slog.Attr
	extractors      []Extractor
	filter          []FilterOption
	fieldExtractors []FieldExtractor
	dropFilters     []DropFilter
}

// WithExtractor appends attrs extracted from each log call context.
// FieldExtractor extracts logx fields from a log call context.
type FieldExtractor func(context.Context) []Field

// DropFilter returns true when a structured log entry should be dropped.
type DropFilter func(context.Context, Entry) bool

func WithExtractor(extractors ...Extractor) Option {
	return func(c *handlerConfig) {
		for _, e := range extractors {
			if e != nil {
				c.extractors = append(c.extractors, e)
			}
		}
	}
}

// WithFieldExtractor appends business-facing field extractors used by New/NewSlog.
func WithFieldExtractor(extractors ...FieldExtractor) Option {
	return func(c *handlerConfig) {
		for _, e := range extractors {
			if e != nil {
				c.fieldExtractors = append(c.fieldExtractors, e)
			}
		}
	}
}

// WithDropFilter appends entry drop filters used by New/NewSlog.
func WithDropFilter(filters ...DropFilter) Option {
	return func(c *handlerConfig) {
		for _, f := range filters {
			if f != nil {
				c.dropFilters = append(c.dropFilters, f)
			}
		}
	}
}

// WithWriter sets the destination writer for the base handler. Defaults to
// [os.Stderr].
func WithWriter(w io.Writer) Option {
	return func(c *handlerConfig) { c.writer = w }
}

// WithFormat selects between text and JSON encoding. Defaults to [FormatText].
func WithFormat(f Format) Option {
	return func(c *handlerConfig) { c.format = f }
}

// WithLevel sets the minimum level for the base handler.
func WithLevel(l Leveler) Option {
	return func(c *handlerConfig) { c.level = l }
}

// WithAddSource toggles inclusion of the source file/line.
func WithAddSource(b bool) Option {
	return func(c *handlerConfig) { c.addSource = b }
}

// WithReplaceAttr installs a custom ReplaceAttr on the base handler.
func WithReplaceAttr(fn func(groups []string, a slog.Attr) slog.Attr) Option {
	return func(c *handlerConfig) { c.replaceAttr = fn }
}

// WithFilter applies the provided filter options on top of the composed
// handler.
func WithFilter(opts ...FilterOption) Option {
	return func(c *handlerConfig) { c.filter = append(c.filter, opts...) }
}

// NewHandler builds a composed [slog.Handler] with kernel defaults:
//   - text encoding to stderr at LevelInfo
//   - context attrs from [ContextWithAttrs] are merged in
//
// Additional decorators are layered as configured.
func NewHandler(opts ...Option) slog.Handler {
	cfg := &handlerConfig{
		writer:     os.Stderr,
		format:     FormatText,
		level:      LevelInfo,
		extractors: []Extractor{AttrsFromContext},
	}
	for _, o := range opts {
		o(cfg)
	}
	h := newBaseHandler(cfg)
	return newComposedHandler(h, cfg)
}

// NewLogger returns a slog logger backed by handler with kernel decorators
// applied.
func NewLogger(handler slog.Handler, opts ...Option) *slog.Logger {
	cfg := &handlerConfig{
		extractors: []Extractor{AttrsFromContext},
	}
	for _, o := range opts {
		o(cfg)
	}
	return slog.New(newComposedHandler(handler, cfg))
}

func newComposedHandler(h slog.Handler, cfg *handlerConfig) slog.Handler {
	if len(cfg.filter) > 0 {
		h = newFilterHandler(h, cfg.filter...)
	}
	return newContextHandler(h, cfg.extractors...)
}

func newBaseHandler(cfg *handlerConfig) slog.Handler {
	hopts := &slog.HandlerOptions{
		Level:       cfg.level,
		AddSource:   cfg.addSource,
		ReplaceAttr: cfg.replaceAttr,
	}
	switch normalizeFormat(cfg.format) {
	case FormatJSON:
		return slog.NewJSONHandler(cfg.writer, hopts)
	default:
		return slog.NewTextHandler(cfg.writer, hopts)
	}
}
