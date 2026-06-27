// Package logx is the Aisphere Kernel logging package.
//
// logx is the ONLY logger that business code (handler/service/repository/worker)
// should use. Direct usage of log.Printf, fmt.Println, log/slog, or
// go.uber.org/zap in business code is forbidden and will fail CI.
//
// # Design principle
//
// logx keeps log/slog as the internal engine (so Kernel is stdlib-only), but
// exposes a Kernel-facing API that does NOT leak slog.Logger or slog.Attr to
// business packages. Other Kernel modules (errorx, httpx, grpcx, auditx,
// metricsx) consume logx through the stable Logger interface + Field type.
//
// logx does NOT call os.Exit, does NOT write to network, does NOT do
// audit-grade recording. It only emits structured logs; other modules consume.
//
// # 30-second quickstart
//
//	cfg := logx.DefaultConfig("dev")
//	cfg.ServiceName = "aihub"
//	logger, levelCtl, err := logx.New(cfg)
//	if err != nil { panic(err) }
//	defer logger.Sync()
//
//	logger.Info("service started",
//	    logx.String("addr", ":8000"),
//	    logx.String("version", "v0.1.0"),
//	)
//
//	_ = levelCtl.SetLevel("debug")  // dynamically lower to debug
//
// In a handler with request-scoped context:
//
//	ctx = logx.Inject(ctx, logger,
//	    logx.String("request_id", reqID),
//	    logx.String("trace_id", traceID),
//	    logx.String("subject_id", userID),
//	)
//	logx.FromContext(ctx).Info("request accepted")
//
// # Logger API
//
// Bootstrap:
//
//	logx.New(cfg)              → (Logger, LevelController, error)
//	logx.DefaultConfig(env)    → Config with sane defaults
//	logx.Noop()                → discard all logs (tests, disabled features)
//	logx.FromContext(ctx)      → request-scoped logger (Noop if none)
//	logx.Inject(ctx, logger, fields...) → attach logger + fields to ctx
//	logx.Sync(logger)          → flush buffers before exit
//
// Logger interface methods:
//
//	type Logger interface {
//	    Debug(msg string, fields ...Field)
//	    Info(msg string, fields ...Field)
//	    Warn(msg string, fields ...Field)
//	    Error(msg string, fields ...Field)
//	    With(fields ...Field) Logger    // add persistent fields
//	    Named(name string) Logger       // add module tag
//	    WithContext(ctx context.Context) Logger
//	    Enabled(level LogLevel) bool
//	    Sync() error
//	}
//
// # Field constructors
//
// All structured fields use these constructors — never slog.Any directly:
//
//	logx.String(k, v)         logx.Int(k, v)        logx.Int64(k, v)
//	logx.Uint64(k, v)         logx.Bool(k, v)       logx.Float64(k, v)
//	logx.Duration(k, v)       logx.Time(k, v)       logx.Any(k, v)
//	logx.Event(name)          logx.Err(err)         logx.Group(k, fields...)
//
// logx.Err(err) is special: it auto-extracts error_code, error_reason,
// http_status, grpc_code, retryable from any error implementing those methods
// (works with errorx without importing it — no cycle).
//
// # Pre-built log helpers
//
// For standardized log shapes, use these instead of building fields manually:
//
//	logx.LogAccess(logger, e logx.AccessEvent)       // HTTP access log
//	logx.LogExternalCall(logger, c logx.ExternalCall) // upstream API call
//	logx.LogError(logger, msg, e logx.ErrorLog)      // standardized error log
//	logx.LogAuditHint(logger, h logx.AuditHint)       // audit breadcrumb
//
// # HTTP / RPC middleware
//
//	logx.HTTPAccessLog(base, cfg, opts...) → http middleware
//	logx.Recovery(base, opts...)           → http panic recovery middleware
//	logx.ServerLogging(base, extract)      → RPC server logging middleware
//	logx.ClientLogging(base, extract)      → RPC client logging middleware
//	logx.RPCRecovery(base, handler)        → RPC panic recovery middleware
//	logx.LevelHTTPHandler(controller)      → runtime level control HTTP handler
//
// # Redaction
//
// logx automatically redacts sensitive field keys (password, token, secret,
// authorization, cookie, credential, private_key, api_key, etc.). Customize
// via Config.Redact. NEVER log credentials directly — redaction only works
// through logx.
//
// # Sampling
//
// For high-QPS logs, enable sampling via Config.Sampling. Only logs at or
// below MinLevel are sampled; warn and error are never sampled.
//
// # Test logger
//
// In tests, use logx.NewTestLogger to capture log entries and assert on them:
//
//	logger := logx.NewTestLogger(t)
//	svc := NewService(logger)
//	svc.Create(ctx, req)
//	logger.AssertLogged(t, "skill created", logx.String("skill_id", "skill_001"))
//
// # Forbidden in business code
//
// Handler / service / repository / worker code MUST NOT use:
//
//	log.Printf("...")
//	fmt.Println("...")
//	fmt.Printf("...")
//	slog.Info("...")
//	slog.Default().Info("...")
//	zap.L().Info("...")
//	logrus.Info("...")
//
// Use logx.FromContext(ctx) or an injected logx.Logger instead.
//
// # Further reading
//
// See logx/README.md for the single-source-of-truth user guide, and
// docs/ai/logx.md for the AI coding recipe.
package logx
