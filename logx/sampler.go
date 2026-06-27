package logx

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

func newSamplingHandler(next slog.Handler, cfg SamplingConfig) slog.Handler {
	if cfg.Every <= 0 {
		cfg.Every = 100
	}
	if cfg.First < 0 {
		cfg.First = 0
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Second
	}
	minLevel, err := ParseLogLevel(cfg.MinLevel)
	if err != nil || minLevel == "" {
		minLevel = DebugLevel
	}
	return &samplingHandler{next: next, cfg: cfg, minLevel: minLevel.slogLevel(), counts: make(map[string]int)}
}

type samplingHandler struct {
	next     slog.Handler
	cfg      SamplingConfig
	minLevel slog.Level
	mu       sync.Mutex
	windowAt time.Time
	counts   map[string]int
}

func (h *samplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *samplingHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level > h.minLevel {
		return h.next.Handle(ctx, record)
	}
	if !h.keep(record) {
		return nil
	}
	return h.next.Handle(ctx, record)
}

func (h *samplingHandler) keep(record slog.Record) bool {
	now := time.Now()
	key := fmt.Sprintf("%d:%s", record.Level, record.Message)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.windowAt.IsZero() || now.Sub(h.windowAt) > h.cfg.Window {
		h.windowAt = now
		h.counts = make(map[string]int)
	}
	h.counts[key]++
	count := h.counts[key]
	if count <= h.cfg.First {
		return true
	}
	return (count-h.cfg.First)%h.cfg.Every == 0
}

func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &samplingHandler{next: h.next.WithAttrs(attrs), cfg: h.cfg, minLevel: h.minLevel, counts: make(map[string]int)}
}

func (h *samplingHandler) WithGroup(name string) slog.Handler {
	return &samplingHandler{next: h.next.WithGroup(name), cfg: h.cfg, minLevel: h.minLevel, counts: make(map[string]int)}
}
