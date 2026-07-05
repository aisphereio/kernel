// Package main demonstrates logx basic usage.
//
// Run:
//
//	go run ./examples/logx-basic
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aisphereio/kernel/logx"
)

func main() {
	// 1. Bootstrap logger with dev config (console format, addSource=true)
	cfg := logx.DefaultConfig("dev")
	cfg.ServiceName = "aihub"
	cfg.Version = "v1.0.0"
	logger, levelCtl, err := logx.New(cfg)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// 2. Basic structured logging
	logger.Info("service started",
		logx.String("addr", ":8000"),
		logx.String("env", cfg.Env),
	)

	// 3. Named logger for a module
	repoLogger := logger.Named("skill_repo")
	repoLogger.Debug("querying skill", logx.String("id", "skill_001"))

	// 4. Persistent fields via With
	authLogger := logger.With(logx.String("component", "auth"))
	authLogger.Info("user authenticated", logx.String("subject_id", "u_123"))

	// 5. Error log with auto-extracted fields
	err = errors.New("pq: connection refused")
	logger.Error("db query failed",
		logx.String("operation", "aihub.skill.query"),
		logx.Err(err),
	)

	// 6. Context-scoped logger (request simulation)
	ctx := logx.Inject(context.Background(), logger,
		logx.String("request_id", "req_abc"),
		logx.String("trace_id", "trace_xyz"),
		logx.String("subject_id", "u_123"),
	)
	logx.FromContext(ctx).Info("request accepted")

	// 7. Pre-built helpers
	logx.LogExternalCall(logger, logx.ExternalCall{
		Provider:   "openai",
		Service:    "chat-completions",
		Operation:  "create",
		Model:      "gpt-4",
		StatusCode: 200,
		Latency:    850 * time.Millisecond,
	})

	logx.LogAuditHint(logger, logx.AuditHint{
		Action:       "aihub.skill.create",
		ActorID:      "u_123",
		ResourceType: "skill",
		ResourceID:   "skill_001",
		Result:       "success",
	})

	// 8. Dynamic level control
	fmt.Println("current level:", levelCtl.GetLevel())
	_ = levelCtl.SetLevel("debug")
	fmt.Println("after SetLevel(debug):", levelCtl.GetLevel())

	// 9. Noop logger (for disabled features or tests)
	noopLogger := logx.Noop()
	noopLogger.Info("this is discarded") // no-op
	fmt.Println("noop enabled?", noopLogger.Enabled(logx.ErrorLevel))
}
