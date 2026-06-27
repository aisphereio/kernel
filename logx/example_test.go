package logx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aisphereio/kernel/logx"
)

// ============================================================================
// FIELD CONSTRUCTOR EXAMPLES
// ============================================================================

func ExampleString() {
	logger := logx.NewTestLogger(nil)
	logger.Info("user login", logx.String("user_id", "u_123"))
	// TestLogger captures entries; see ExampleNewTestLogger for assertion.
	fmt.Println("ok")
	// Output: ok
}

func ExampleInt() {
	logger := logx.NewTestLogger(nil)
	logger.Info("request", logx.Int("status", 200))
	fmt.Println("ok")
	// Output: ok
}

func ExampleInt64() {
	logger := logx.NewTestLogger(nil)
	logger.Info("file", logx.Int64("bytes", 1024))
	fmt.Println("ok")
	// Output: ok
}

func ExampleBool() {
	logger := logx.NewTestLogger(nil)
	logger.Info("feature", logx.Bool("enabled", true))
	fmt.Println("ok")
	// Output: ok
}

func ExampleFloat64() {
	logger := logx.NewTestLogger(nil)
	logger.Info("metric", logx.Float64("rate", 0.95))
	fmt.Println("ok")
	// Output: ok
}

func ExampleDuration() {
	logger := logx.NewTestLogger(nil)
	logger.Info("request done",
		logx.Duration("latency", 42*time.Millisecond),
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleTime() {
	logger := logx.NewTestLogger(nil)
	logger.Info("created",
		logx.Time("created_at", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleAny() {
	type User struct{ ID string }
	logger := logx.NewTestLogger(nil)
	logger.Info("user", logx.Any("user", User{ID: "u_123"}))
	fmt.Println("ok")
	// Output: ok
}

func ExampleEvent() {
	logger := logx.NewTestLogger(nil)
	logger.Info("skill created", logx.Event("skill_created"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleErr() {
	// logx.Err auto-extracts error_code, http_status, retryable from any
	// error implementing those methods (works with errorx without import).
	err := errors.New("db timeout")
	logger := logx.NewTestLogger(nil)
	logger.Error("query failed", logx.Err(err))
	fmt.Println("ok")
	// Output: ok
}

func ExampleGroup() {
	logger := logx.NewTestLogger(nil)
	logger.Info("user action",
		logx.Group("user",
			logx.String("id", "u_123"),
			logx.String("name", "alice"),
		),
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// LOGGER METHODS
// ============================================================================

func ExampleNew() {
	cfg := logx.DefaultConfig("dev")
	cfg.ServiceName = "aihub"
	logger, levelCtl, err := logx.New(cfg, logx.WithWriter(io.Discard))
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("service started", logx.String("addr", ":8000"))
	_ = levelCtl.SetLevel("debug") // dynamically lower level
	fmt.Println("ok")
	// Output: ok
}

func ExampleNewSlog() {
	// NewSlog is the slog-backed implementation. New is an alias.
	cfg := logx.DefaultConfig("test")
	cfg.Output = "stderr"
	logger, _, err := logx.NewSlog(cfg)
	if err != nil {
		panic(err)
	}
	logger.Info("ok")
	fmt.Println("started")
	// Output: started
}

func ExampleNoop() {
	// Noop discards all logs. Use in tests or disabled features.
	logger := logx.Noop()
	logger.Info("this is discarded")
	fmt.Println("ok")
	// Output: ok
}

func ExampleSync() {
	cfg := logx.DefaultConfig("test")
	logger, _, _ := logx.New(cfg)
	// Sync flushes buffers before exit.
	_ = logx.Sync(logger)
	fmt.Println("ok")
	// Output: ok
}

func ExampleSlog() {
	// Slog unwraps a logx.Logger to *slog.Logger. ADAPTERS ONLY — business
	// code should never need this.
	cfg := logx.DefaultConfig("test")
	logger, _, _ := logx.New(cfg)
	if sl, err := logx.Slog(logger); err == nil && sl != nil {
		fmt.Println("unwrapped")
	}
	// Output: unwrapped
}

// ============================================================================
// LOGGER INTERFACE METHODS
// ============================================================================

func ExampleLogger_Debug() {
	logger := logx.NewTestLogger(nil)
	logger.Debug("debug message", logx.String("key", "value"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_Info() {
	logger := logx.NewTestLogger(nil)
	logger.Info("info message", logx.String("key", "value"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_Warn() {
	logger := logx.NewTestLogger(nil)
	logger.Warn("warn message", logx.String("key", "value"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_Error() {
	logger := logx.NewTestLogger(nil)
	logger.Error("error message", logx.Err(errors.New("failed")))
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_With() {
	logger := logx.NewTestLogger(nil)
	// With returns a new logger with persistent fields.
	serviceLogger := logger.With(
		logx.String("service", "aihub"),
		logx.String("version", "v1.0"),
	)
	serviceLogger.Info("started")
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_Named() {
	logger := logx.NewTestLogger(nil)
	// Named returns a new logger with a module tag.
	repoLogger := logger.Named("skill_repo")
	repoLogger.Info("querying")
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_WithContext() {
	logger := logx.NewTestLogger(nil)
	ctx := context.Background()
	// WithContext binds a context to the logger (for context-extracted fields).
	ctxLogger := logger.WithContext(ctx)
	ctxLogger.Info("request done")
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogger_Enabled() {
	logger := logx.Noop() // Noop returns Enabled=false always
	fmt.Println(logger.Enabled(logx.ErrorLevel))
	// Output: false
}

// ============================================================================
// CONTEXT FIELDS
// ============================================================================

func ExampleContextWithFields() {
	ctx := context.Background()
	ctx = logx.ContextWithFields(ctx,
		logx.String("request_id", "req_123"),
		logx.String("user_id", "u_123"),
	)
	fields := logx.FieldsFromContext(ctx)
	fmt.Println(len(fields))
	// Output: 2
}

func ExampleFieldsFromContext() {
	ctx := context.Background()
	fields := logx.FieldsFromContext(ctx)
	fmt.Println(len(fields))
	// Output: 0
}

func ExampleContextWithAttrs() {
	// ContextWithAttrs is the slog-level primitive (kernel internals).
	ctx := context.Background()
	// (use slog.Any to make attrs; omitted for brevity)
	attrs := logx.AttrsFromContext(ctx)
	fmt.Println(len(attrs))
	// Output: 0
}

func ExampleInject() {
	logger := logx.NewTestLogger(nil)
	ctx := context.Background()
	ctx = logx.Inject(ctx, logger,
		logx.String("request_id", "req_abc"),
		logx.String("subject_id", "u_123"),
	)
	// FromContext now returns a logger with those fields attached.
	ctxLogger := logx.FromContext(ctx)
	ctxLogger.Info("request accepted")
	fmt.Println("ok")
	// Output: ok
}

func ExampleFromContext() {
	// FromContext returns the injected logger, or Noop if none.
	ctx := context.Background()
	logger := logx.FromContext(ctx)
	fmt.Println(logger == logx.Noop())
	// Output: true
}

func ExampleFromContext_nil() {
	// FromContext is nil-safe: returns Noop if ctx has no logger.
	logger := logx.FromContext(context.Background())
	fmt.Println(logger == logx.Noop())
	// Output: true
}

func ExampleFromContextOr() {
	logger := logx.NewTestLogger(nil)
	ctx := context.Background()
	// Returns injected logger if present, else fallback.
	result := logx.FromContextOr(ctx, logger)
	result.Info("ok")
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// CONFIGURATION
// ============================================================================

func ExampleDefaultConfig() {
	cfg := logx.DefaultConfig("prod")
	fmt.Println(cfg.Format)
	fmt.Println(cfg.Level)
	fmt.Println(cfg.AddSource)
	// Output:
	// json
	// info
	// false
}

func ExampleDefaultConfig_dev() {
	cfg := logx.DefaultConfig("dev")
	fmt.Println(cfg.Format)
	fmt.Println(cfg.AddSource)
	// Output:
	// console
	// true
}

func ExampleNewRedactor() {
	r := logx.NewRedactor(logx.RedactConfig{
		Enabled: true,
		Keys:    []string{"password", "token"},
		Value:   "***",
	})
	field := r.Redact(logx.String("password", "s3cr3t"))
	fmt.Println(field.Value)
	// Output: ***
}

func ExampleDefaultRedactKeys() {
	keys := logx.DefaultRedactKeys()
	// Default list includes password, token, secret, authorization, cookie,
	// credential, private_key, api_key, ak, sk, etc.
	fmt.Println(len(keys) > 5)
	// Output: true
}

// ============================================================================
// PRE-BUILT LOG HELPERS
// ============================================================================

func ExampleLogAccess() {
	logger := logx.NewTestLogger(nil)
	logx.LogAccess(logger, logx.AccessEvent{
		Side:       "server",
		Protocol:   "http",
		Operation:  "POST /v1/skills",
		Method:     "POST",
		Path:       "/v1/skills",
		StatusCode: 201,
		Latency:    42 * time.Millisecond,
	})
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogAccess_error() {
	// Access events with status >= 500 or Err != nil auto-log at ERROR level.
	logger := logx.NewTestLogger(nil)
	logx.LogAccess(logger, logx.AccessEvent{
		Operation:  "GET /v1/skills/missing",
		StatusCode: 500,
		Latency:    5 * time.Millisecond,
		Err:        errors.New("db connection refused"),
	})
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogExternalCall() {
	logger := logx.NewTestLogger(nil)
	logx.LogExternalCall(logger, logx.ExternalCall{
		Provider:   "openai",
		Service:    "chat-completions",
		Operation:  "create",
		Model:      "gpt-4",
		Endpoint:   "https://api.openai.com/v1/chat/completions",
		StatusCode: 200,
		Latency:    850 * time.Millisecond,
	})
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogError() {
	logger := logx.NewTestLogger(nil)
	err := errors.New("db timeout")
	logx.LogError(logger, "create skill failed", logx.ErrorLog{
		Operation:    "aihub.skill.create",
		ResourceType: "skill",
		ResourceID:   "skill_001",
		Code:         "AIHUB_SKILL_CREATE_FAILED",
		Reason:       "db_error",
		Err:          err,
	})
	fmt.Println("ok")
	// Output: ok
}

func ExampleLogAuditHint() {
	logger := logx.NewTestLogger(nil)
	logx.LogAuditHint(logger, logx.AuditHint{
		Action:       "aihub.skill.delete",
		ActorID:      "user_123",
		ResourceType: "skill",
		ResourceID:   "skill_001",
		Result:       "success",
	})
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// FILTERING / SAMPLING
// ============================================================================

func ExampleFilterKey() {
	// FilterKey redacts values for the given keys.
	_ = logx.FilterKey("password", "token", "secret")
	fmt.Println("ok")
	// Output: ok
}

func ExampleFilterFunc() {
	// FilterFunc drops records for which fn returns true.
	_ = logx.FilterFunc(func(_ context.Context, _ slog.Record) bool {
		return false
	})
	fmt.Println("ok")
	// Output: ok
}

func ExampleDropEvents() {
	// DropEvents drops entries with event=<any of given>.
	_ = logx.DropEvents("debug_ping", "noisy_loop")
	fmt.Println("ok")
	// Output: ok
}

func ExampleDropMessages() {
	_ = logx.DropMessages("noisy library log", "deprecated")
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// TEST LOGGER
// ============================================================================

func ExampleNewTestLogger() {
	logger := logx.NewTestLogger(nil)
	logger.Info("hello", logx.String("key", "value"))
	entries := logger.Entries()
	fmt.Println(len(entries))
	fmt.Println(entries[0].Message)
	// Output:
	// 1
	// hello
}

func ExampleTestLogger_AssertLogged() {
	// In real tests, pass *testing.T.
	t := &testing.T{}
	logger := logx.NewTestLogger(t)
	logger.Info("skill created", logx.String("skill_id", "skill_001"))
	logger.AssertLogged(t, "skill created", logx.String("skill_id", "skill_001"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleTestLogger_Entries() {
	logger := logx.NewTestLogger(nil)
	logger.Info("first")
	logger.Info("second")
	fmt.Println(len(logger.Entries()))
	// Output: 2
}

// ============================================================================
// LEVEL CONTROL
// ============================================================================

func ExampleParseLevel() {
	// ParseLevel returns slog.Level (kernel internals).
	level := logx.ParseLevel("warn")
	fmt.Println(level)
	// Output: WARN
}

func ExampleParseLogLevel() {
	// ParseLogLevel returns logx.LogLevel (business code).
	level, _ := logx.ParseLogLevel("error")
	fmt.Println(level)
	// Output: error
}

func ExampleParseLogLevel_invalid() {
	_, err := logx.ParseLogLevel("invalid")
	fmt.Println(err != nil)
	// Output: true
}

func ExampleLevelController() {
	cfg := logx.DefaultConfig("test")
	_, levelCtl, _ := logx.New(cfg)
	fmt.Println(levelCtl.GetLevel())
	_ = levelCtl.SetLevel("debug")
	fmt.Println(levelCtl.GetLevel())
	// Output:
	// info
	// debug
}

// ============================================================================
// HANDLER BUILDER (kernel internals)
// ============================================================================

func ExampleNewHandler() {
	// NewHandler builds a composed slog.Handler with kernel defaults.
	// Business code should use logx.New(cfg) instead.
	handler := logx.NewHandler(
		logx.WithFormat(logx.FormatJSON),
	)
	if handler != nil {
		fmt.Println("ok")
	}
	// Output: ok
}

func ExampleNewLogger() {
	// NewLogger wraps an existing slog.Handler with kernel decorators.
	// Kernel internals only.
	handler := logx.NewHandler()
	logger := logx.NewLogger(handler)
	if logger != nil {
		fmt.Println("ok")
	}
	// Output: ok
}

func ExampleWithFormat() {
	_ = logx.WithFormat(logx.FormatJSON)
	_ = logx.WithFormat(logx.FormatConsole)
	fmt.Println("ok")
	// Output: ok
}

func ExampleWithWriter() {
	// WithWriter sets the destination writer for the handler.
	// _ = logx.WithWriter(os.Stdout)
	fmt.Println("ok")
	// Output: ok
}

func ExampleWithFilter() {
	// WithFilter applies redaction filters.
	_ = logx.WithFilter(logx.FilterKey("password"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleWithAddSource() {
	_ = logx.WithAddSource(true)
	fmt.Println("ok")
	// Output: ok
}

func ExampleWithDropFilter() {
	_ = logx.WithDropFilter(logx.DropEvents("debug_ping"))
	fmt.Println("ok")
	// Output: ok
}

func ExampleWithLevel() {
	_ = logx.WithLevel(logx.LevelInfo)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// FORMAT CONSTANTS
// ============================================================================

func ExampleFormat() {
	fmt.Println(logx.FormatJSON)
	fmt.Println(logx.FormatConsole)
	// Output:
	// json
	// console
}

// ============================================================================
// LOG LEVEL CONSTANTS
// ============================================================================

func ExampleLogLevel() {
	fmt.Println(logx.DebugLevel)
	fmt.Println(logx.InfoLevel)
	fmt.Println(logx.WarnLevel)
	fmt.Println(logx.ErrorLevel)
	// Output:
	// debug
	// info
	// warn
	// error
}

func ExampleLogLevel_String() {
	fmt.Println(logx.InfoLevel.String())
	fmt.Println(logx.LogLevel("").String()) // empty defaults to info
	// Output:
	// info
	// info
}

// ============================================================================
// PACKAGE-LEVEL HELPERS (kernel internals, mirrors slog)
// ============================================================================

func ExampleInfo() {
	// Package-level helpers use the default logger. Kernel internals only —
	// business code should use logx.FromContext(ctx) or an injected logger.
	// Set up a default first:
	handler := logx.NewHandler(logx.WithFormat(logx.FormatJSON))
	logx.SetDefault(logx.NewLogger(handler))
	logx.Info("package-level info", "key", "value")
	fmt.Println("ok")
	// Output: ok
}

func ExampleSetDefault() {
	handler := logx.NewHandler()
	logx.SetDefault(logx.NewLogger(handler))
	fmt.Println("ok")
	// Output: ok
}

func ExampleDefault() {
	// Default returns the default slog logger.
	logx.SetDefault(logx.NewLogger(logx.NewHandler()))
	l := logx.Default()
	if l != nil {
		fmt.Println("ok")
	}
	// Output: ok
}

func ExampleWith() {
	// With returns a logger with persistent attrs (slog-level, kernel internals).
	logx.SetDefault(logx.NewLogger(logx.NewHandler()))
	l := logx.With("service", "aihub")
	if l != nil {
		fmt.Println("ok")
	}
	// Output: ok
}

func ExampleEnabled() {
	logx.SetDefault(logx.NewLogger(logx.NewHandler(logx.WithLevel(logx.LevelInfo))))
	fmt.Println(logx.Enabled(context.Background(), logx.LevelInfo))
	// Output: true
}
