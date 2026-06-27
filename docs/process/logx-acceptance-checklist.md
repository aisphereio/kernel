# logx Acceptance Checklist

## Static checks

- [ ] `grep -R "log\.Printf\|fmt\.Println\|fmt\.Printf" --include="*.go" . | grep -v _test.go | grep -v examples/ | grep -v cmd/` returns no business code violations
- [ ] `grep -R '"log/slog"' --include="*.go" . | grep -v _test.go | grep -v logx/ | grep -v examples/ | grep -v cmd/ | grep -v internal/` returns no business code violations
- [ ] `grep -R '"go.uber.org/zap"' --include="*.go" . | grep -v _test.go | grep -v logx/` returns no business code violations
- [ ] `logx/README.md` exists, 200+ lines
- [ ] `logx/doc.go` exists, 80+ lines
- [ ] `logx/example_test.go` exists with ExampleXxx for every public function
- [ ] `logx/example_business_test.go` exists with 10 business scenarios
- [ ] `docs/ai/logx.md` exists as single AI recipe (not split)
- [ ] `.cursor/rules/logx.mdc` exists
- [ ] `CLAUDE.md` / `AGENTS.md` / `.github/copilot-instructions.md` mention logx
- [ ] `docs/design/logx.md` exists (already 1307 lines)
- [ ] `docs/contracts/logx.md` exists
- [ ] Root `README.md` lists logx in module status table
- [ ] `docs/README.md` lists logx in navigation index

## Unit checks

Run:

```bash
go test ./logx -v
go test ./logx -race
go test ./logx -cover
go test ./logx -bench=.
```

Expected coverage areas:

- Logger interface (Debug/Info/Warn/Error/With/Named/WithContext/Enabled/Sync)
- Field constructors (String/Int/Int64/Uint64/Bool/Float64/Duration/Time/Any/Event/Err/Group)
- Context fields (ContextWithFields/FieldsFromContext/Inject/FromContext/FromContextOr)
- nil safety (FromContext(nil), Inject(nil, ...), Noop())
- Redaction (default keys, custom keys, disabled)
- Sampling (Every/First/Window/MinLevel)
- Filtering (FilterKey, FilterFunc, DropEvents, DropMessages)
- Pre-built helpers (LogAccess, LogExternalCall, LogError, LogAuditHint)
- HTTP middleware (HTTPAccessLog, Recovery)
- RPC middleware (ServerLogging, ClientLogging, RPCRecovery)
- Level control (ParseLevel, ParseLogLevel, LevelController)
- Test logger (NewTestLogger, AssertLogged, Entries)
- Format (JSON, Console, normalizeFormat)
- DefaultConfig for dev/staging/prod

## Integration checks

Run on a full local machine with Go 1.25+:

```bash
go test ./...
go vet ./...
go run ./examples/logx-basic
go run ./examples/logx-http &
curl -i 'http://localhost:18080/?status=200'
curl -i 'http://localhost:18080/?status=500'
curl -i 'http://localhost:18080/panic'
kill %1
```

Expected scenarios:

1. `logx.New(cfg)` returns non-nil logger, LevelController, nil error
2. `logx.DefaultConfig("dev")` returns console format, AddSource=true
3. `logx.DefaultConfig("prod")` returns JSON format, AddSource=false
4. `logx.FromContext(ctx)` returns Noop if no logger injected
5. `logx.FromContext(ctx)` returns injected logger with fields attached
6. `logx.Err(err)` extracts error_code/http_status/retryable from errorx-style errors
7. `logx.LogAccess` auto-selects level by status code (200→Info, 404→Warn, 500→Error)
8. `logx.LogExternalCall` same auto-level behavior
9. `logx.HTTPAccessLog` middleware produces one access log per request
10. `logx.Recovery` middleware recovers panics and logs at Error level
11. Redaction replaces sensitive keys with `***`
12. Sampling drops debug/info logs beyond First+Every ratio; warn/error never sampled
13. `logx.NewTestLogger(t).AssertLogged` passes for matched entries, fails for unmatched
14. `LevelController.SetLevel("debug")` dynamically lowers level
15. `LevelHTTPHandler` exposes `/admin/log-level?level=debug` endpoint

## Forbidden import check (golangci-lint depguard)

```bash
golangci-lint run
```

Expected: no violations in handler/service/repository/worker code:

```text
handler/*.go     — must not import "log", "fmt" (for logging), "log/slog", "go.uber.org/zap", "github.com/sirupsen/logrus"
service/*.go     — same
repository/*.go  — same
worker/*.go      — same
```

Exempt paths:

- `logx/` (the package itself)
- `*_test.go` (tests can use anything)
- `examples/` (demos for independence)
- `cmd/` / `internal/` (kernel internals)
- `contrib/` (external integrations)

## Windows commands

Windows can use Make directly:

```powershell
make test
make vet
```

Or run the wrapper scripts directly:

```bat
scripts\test-cmd.cmd
```

## CI integration

Add to `.github/workflows/logx-enforcement.yml` (optional but recommended):

```yaml
name: logx enforcement
on: [pull_request, push]
jobs:
  logx-usage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Check logx usage
        run: |
          # grep for forbidden patterns in business code
          ! grep -rE 'log\.(Printf|Println|Print)\(' --include="*.go" . \
            | grep -v _test.go | grep -v examples/ | grep -v cmd/ | grep -v internal/ \
            | grep -v logx/
          ! grep -rE 'fmt\.(Println|Printf)\(' --include="*.go" . \
            | grep -v _test.go | grep -v examples/ | grep -v cmd/ | grep -v internal/ \
            | grep -v logx/
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.25" }
      - run: go test ./logx -race
      - run: go test ./...
```
