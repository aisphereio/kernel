package logx

import (
	"fmt"
	"log/slog"
	"strings"
)

// LogLevel is the Kernel business-facing log level. It intentionally stays
// independent from slog.Level so service code can depend on logx without
// importing the concrete slog engine.
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return InfoLevel, nil
	case "debug":
		return DebugLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	default:
		return "", fmt.Errorf("unsupported log level %q", s)
	}
}

func (l LogLevel) String() string {
	if l == "" {
		return string(InfoLevel)
	}
	return string(l)
}

func (l LogLevel) slogLevel() slog.Level {
	switch l {
	case DebugLevel:
		return slog.LevelDebug
	case WarnLevel:
		return slog.LevelWarn
	case ErrorLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func logLevelFromSlog(level slog.Level) LogLevel {
	switch {
	case level <= slog.LevelDebug:
		return DebugLevel
	case level >= slog.LevelError:
		return ErrorLevel
	case level >= slog.LevelWarn:
		return WarnLevel
	default:
		return InfoLevel
	}
}

// LevelController controls log level at runtime. The slog implementation is
// backed by slog.LevelVar, which is safe for concurrent use.
type LevelController interface {
	GetLevel() string
	SetLevel(level string) error
}

type slogLevelController struct {
	level *slog.LevelVar
}

func (c slogLevelController) GetLevel() string {
	if c.level == nil {
		return InfoLevel.String()
	}
	return strings.ToLower(c.level.Level().String())
}

func (c slogLevelController) SetLevel(level string) error {
	parsed, err := ParseLogLevel(level)
	if err != nil {
		return err
	}
	if c.level != nil {
		c.level.Set(parsed.slogLevel())
	}
	return nil
}
