# Example Coverage Standard

How to measure and achieve Example coverage for a kernel module. This is
Dimension 3 of the completeness standard.

## Why Example coverage matters

AI tools (Claude Code, Cursor, Copilot) learn from Examples. If a public
API has no `ExampleXxx` function:

1. `go doc <module>.<Function>` returns no example → developer disappointed
2. AI tools reading `CLAUDE.md` Example table won't find it → AI improvises
3. Future maintainers don't know the intended usage → API misused

**Target: 90%+ Example coverage.** Below 70%, the module is not doc-complete.

---

## Coverage formula

```text
Public API surface = constructors + options + inspect functions + predicates

Example coverage = (APIs with ExampleXxx) / (Public API surface) × 100%
```

### What counts as "public API"

| Category | How to identify | Example |
|---|---|---|
| Constructor | `func [A-Z]...(` exported, returns `*Type` or `error` | `NotFound`, `BadRequest` |
| Option | `func With[A-Z]...(` exported | `WithMetadata`, `WithRetryable` |
| Inspect function | `func [A-Z]...(err error)` or `func [A-Z]...Of(` | `CodeOf`, `HTTPStatusOf` |
| Predicate | `func Is[A-Z]...(` exported | `IsNotFound`, `IsCode` |
| Method on *Type | `func (*Type) [A-Z]...(` exported | `(*Error).Clone`, `(*Error).Code` |

### What doesn't count

- Private functions (lowercase)
- Test helpers
- Generated code (`*.pb.go`)
- Internal packages (`internal/`)

---

## Minimum Example set

For a module with N public APIs, the minimum Example set is:

```text
Example count = N (one per API) + 10 (business scenarios) + 5 (advanced)
             = N + 15
```

For errorx (50 public APIs): 50 + 15 = 65 Examples (we have 55, close to target).
For logx (~30 public APIs): 30 + 15 = 45 Examples minimum.
For httpx (~40 public APIs): 40 + 15 = 55 Examples minimum.

---

## The 10 standard business scenarios

Every module's `example_business_test.go` should include these 10 scenarios,
adapted to the module's domain. These cover the full handler → service →
repository → consumer flow.

### Scenario 1: Repository layer — convert storage errors

Shows how repository converts DB/storage errors to module errors.

```go
func ExampleBusiness_repositoryLayer() {
    repo := &fakeRepo{}
    _, err := repo.Find(context.Background(), "missing")
    fmt.Println(module.CodeOf(err))
    // Output: DOMAIN_RESOURCE_NOT_FOUND
}
```

### Scenario 2: Service layer — validation + business rules

Shows service-layer validation, conflict detection, all returning module errors.

```go
func ExampleBusiness_serviceLayer() {
    svc := &fakeService{}
    _, err := svc.Create(context.Background(), "")
    fmt.Println(module.CodeOf(err))
    // Output: DOMAIN_RESOURCE_NAME_REQUIRED
}
```

### Scenario 3: Upstream dependency failure

Shows wrapping upstream API/service failures with retryable flag.

```go
func ExampleBusiness_upstreamTimeout() {
    err := callUpstream(context.Background())
    fmt.Println(module.CodeOf(err))
    fmt.Println(module.RetryableOf(err))
    // Output:
    // UPSTREAM_TIMEOUT
    // true
}
```

### Scenario 4: Authz denied

Shows permission denied with resource/action in metadata.

```go
func ExampleBusiness_authzDenied() {
    err := checkPermission(ctx, "user_123", "resource", "action")
    fmt.Println(module.CodeOf(err))
    fmt.Println(module.MetadataOf(err)["subject_id"])
    // Output:
    // PERMISSION_DENIED
    // user_123
}
```

### Scenario 5: Worker retry decision

Shows how a worker uses module's retryable flag to decide retry vs fail.

```go
func ExampleBusiness_workerRetry() {
    cases := []struct{ name string; err error }{
        {"timeout", module.Timeout("UPSTREAM_TIMEOUT", "x", module.WithRetryable(true))},
        {"validation", module.BadRequest("INVALID", "x")},
    }
    for _, c := range cases {
        action := "fail"
        if module.RetryableOf(c.err) { action = "retry" }
        fmt.Printf("%s -> %s\n", c.name, action)
    }
    // Output:
    // timeout -> retry
    // validation -> fail
}
```

### Scenario 6: HTTP response shape

Shows the JSON shape that httpx middleware produces from a module error.

```go
func ExampleBusiness_httpResponse() {
    err := module.NotFound("CODE", "msg",
        module.WithPublicMetadata("resource", "skill"),
        module.WithRequestID("req_abc"),
    )
    resp := map[string]any{
        "code":       module.CodeOf(err).String(),
        "message":    module.MessageOf(err),
        "request_id": module.RequestIDOf(err),
        "metadata":   module.PublicMetadataOf(err),
    }
    fmt.Println(resp["code"])
    // Output: CODE
}
```

### Scenario 7: Audit record

Shows what auditx records from a module error.

```go
func ExampleBusiness_auditRecord() {
    err := module.Forbidden("DENIED", "no permission",
        module.WithMetadata("subject_id", "user_123"),
        module.WithRequestID("req_xyz"),
    )
    audit := map[string]string{
        "result":     "deny",
        "error_code": module.CodeOf(err).String(),
        "request_id": module.RequestIDOf(err),
    }
    fmt.Println(audit["result"])
    // Output: deny
}
```

### Scenario 8: Log entry (with redaction)

Shows what logx records, including auto-redaction of sensitive metadata.

```go
func ExampleBusiness_logEntry() {
    err := module.Internal("FAILED", "x",
        module.WithMetadata("password", "s3cr3t"),
        module.WithMetadata("dsn", "postgres://..."),
    )
    fields := module.Fields(err)
    safeMD := fields["error_metadata"].(map[string]any)
    fmt.Println(safeMD["password"])  // [REDACTED]
    fmt.Println(safeMD["dsn"])       // postgres://...
    // Output:
    // [REDACTED]
    // postgres://...
}
```

### Scenario 9: Metrics labels (low cardinality)

Shows what metricsx emits — ONLY low-cardinality fields.

```go
func ExampleBusiness_metricsLabels() {
    err := module.Timeout("UPSTREAM_TIMEOUT", "x",
        module.WithMetadata("model", "gpt-4"),     // dynamic — NOT in labels
        module.WithRequestID("req_abc"),           // dynamic — NOT in labels
    )
    labels := module.MetricsLabels(err)
    fmt.Println(labels["error_code"])
    fmt.Println(labels["retryable"])
    // Output:
    // UPSTREAM_TIMEOUT
    // true
}
```

### Scenario 10: Multi-layer wrap (preserve chain)

Shows errors propagating through layers while preserving the original cause.

```go
func ExampleBusiness_multiLayerWrap() {
    dbErr := errors.New("pq: connection refused")
    repoErr := module.Wrap(dbErr, "REPO_FAILED",
        module.WithRetryable(true),
    )
    svcErr := module.Wrap(repoErr, "SVC_FAILED")

    fmt.Println(errors.Is(svcErr, dbErr))  // true — chain preserved
    fmt.Println(module.CodeOf(svcErr))
    fmt.Println(module.RetryableOf(svcErr))  // true — inherited
    // Output:
    // true
    // SVC_FAILED
    // true
}
```

---

## Advanced Examples (5 standard ones)

In addition to the 10 business scenarios, include these 5 advanced Examples
in `example_test.go`:

### Advanced 1: Clone (deep copy with override)

```go
func ExampleError_Clone() {
    original := module.NotFound("CODE", "msg",
        module.WithMetadata("id", "123"),
    )
    clone := original.Clone()
    // Modify clone without affecting original
    // ...
}
```

### Advanced 2: %+v debug format

```go
func ExampleError_Format() {
    err := module.Internal("CODE", "msg",
        module.WithCause(errors.New("underlying")),
    )
    fmt.Println(err.Error())  // safe message
    // fmt.Printf("%+v\n", err)  // full debug info
    // Output: msg
}
```

### Advanced 3: As (extract from chain)

```go
func ExampleAs() {
    original := module.NotFound("CODE", "msg")
    wrapped := fmt.Errorf("repo: %w", original)

    ke, ok := module.As(wrapped)
    fmt.Println(ok)
    fmt.Println(ke.Code())
    // Output:
    // true
    // CODE
}
```

### Advanced 4: IsKernelError / IsModuleError

```go
func ExampleIsKernelError() {
    fmt.Println(module.IsKernelError(module.NotFound("CODE", "x")))
    fmt.Println(module.IsKernelError(errors.New("plain")))
    // Output:
    // true
    // false
}
```

### Advanced 5: Third-party compatibility

```go
// gofrStyleError mimics GoFr's error type
type gofrStyleError struct{ msg string; status int }
func (e gofrStyleError) Error() string  { return e.msg }
func (e gofrStyleError) StatusCode() int { return e.status }

func ExampleFrom_foreignStatusCode() {
    foreignErr := gofrStyleError{msg: "not found", status: 404}
    ke := module.From(foreignErr)
    fmt.Println(ke.HTTPStatus())
    fmt.Println(ke.Code())
    // Output:
    // 404
    // NOT_FOUND
}
```

---

## nil safety Examples (3 standard ones)

Always include these to prove inspect helpers are nil-safe:

```go
func ExampleCodeOf_nil() {
    fmt.Println(module.CodeOf(nil))
    // Output: OK
}

func ExampleHTTPStatusOf_nil() {
    fmt.Println(module.HTTPStatusOf(nil))
    // Output: 200
}

func ExampleFields_nil() {
    fields := module.Fields(nil)
    fmt.Println(fields["error_code"])
    fmt.Println(fields["http_status"])
    // Output:
    // OK
    // 200
}
```

---

## Verification script

Run this to check Example coverage:

```bash
#!/bin/bash
MODULE=${1:?usage: check-example-coverage.sh <module>}

# Count public APIs
CONSTRUCTORS=$(grep -E '^func [A-Z][a-zA-Z]+\(' $MODULE/*.go 2>/dev/null \
  | grep -v _test.go | grep -v '//' | wc -l)
OPTIONS=$(grep -E '^func With[A-Z][a-zA-Z]+\(' $MODULE/*.go 2>/dev/null \
  | grep -v _test.go | wc -l)
INSPECT=$(grep -E '^func [A-Z][a-zA-Z]+Of\(|^func Is[A-Z][a-zA-Z]+\(' $MODULE/*.go 2>/dev/null \
  | grep -v _test.go | wc -l)
TOTAL_API=$((CONSTRUCTORS + OPTIONS + INSPECT))

# Count Examples
EXAMPLES=$(grep -c '^func Example' $MODULE/example_test.go 2>/dev/null || echo 0)
BUSINESS=$(grep -c '^func ExampleBusiness' $MODULE/example_business_test.go 2>/dev/null || echo 0)
TOTAL_EXAMPLES=$((EXAMPLES + BUSINESS))

# Coverage ratio
if [ $TOTAL_API -gt 0 ]; then
  RATIO=$((EXAMPLES * 100 / TOTAL_API))
else
  RATIO=0
fi

echo "Module: $MODULE"
echo "Public APIs: $TOTAL_API (constructors=$CONSTRUCTORS, options=$OPTIONS, inspect=$INSPECT)"
echo "Examples: $TOTAL_EXAMPLES (test=$EXAMPLES, business=$BUSINESS)"
echo "Coverage ratio: ${RATIO}%"
echo

if [ $RATIO -ge 90 ]; then
  echo "✓ coverage >= 90%"
elif [ $RATIO -ge 70 ]; then
  echo "⚠ coverage 70-89% (needs improvement)"
else
  echo "✗ coverage < 70% (NOT doc-complete)"
fi

if [ $BUSINESS -ge 10 ]; then
  echo "✓ 10+ business scenarios"
else
  echo "✗ only $BUSINESS business scenarios (need 10+)"
fi
```

---

## Common mistakes

### Mistake 1: Examples without `// Output:`

```go
// ❌ BAD — can't be auto-verified
func ExampleNotFound() {
    err := errorx.NotFound("CODE", "msg")
    fmt.Println(errorx.CodeOf(err))
}

// ✅ GOOD — go test verifies the output
func ExampleNotFound() {
    err := errorx.NotFound("CODE", "msg")
    fmt.Println(errorx.CodeOf(err))
    // Output: CODE
}
```

### Mistake 2: Examples that depend on external state

```go
// ❌ BAD — depends on current time, output varies
func ExampleNow() {
    fmt.Println(errorx.WithRequestID(time.Now().String()))
    // Output: <can't predict>
}

// ✅ GOOD — use fixed values
func ExampleWithRequestID() {
    err := errorx.NotFound("CODE", "msg",
        errorx.WithRequestID("req_123"),
    )
    fmt.Println(errorx.RequestIDOf(err))
    // Output: req_123
}
```

### Mistake 3: Examples too trivial to be useful

```go
// ❌ BAD — doesn't show real usage
func ExampleNotFound() {
    _ = errorx.NotFound
    // Output:
}

// ✅ GOOD — shows the function in action
func ExampleNotFound() {
    err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
        errorx.WithMetadata("skill_id", "skill_001"),
    )
    fmt.Println(errorx.CodeOf(err))
    fmt.Println(errorx.HTTPStatusOf(err))
    // Output:
    // AIHUB_SKILL_NOT_FOUND
    // 404
}
```

### Mistake 4: Business scenarios without context

```go
// ❌ BAD — no comment explaining when to use this
func ExampleBusiness_repositoryLayer() {
    // ...code...
}

// ✅ GOOD — comment explains the scenario
// SCENARIO 1: Repository layer — convert DB errors to errorx
// Shows how repository converts sql.ErrNoRows to errorx.NotFound.
func ExampleBusiness_repositoryLayer() {
    // ...code...
}
```

### Mistake 5: Forgetting nil safety Examples

Always include `ExampleXxx_nil` for inspect functions. AI tools need to know
the function is nil-safe.

---

## Worked example: errorx coverage

errorx has:
- 11 constructors → 11 Examples (one each)
- 14 options → 14 Examples (one each)
- 13 inspect functions → 13 Examples + 3 nil-safety Examples
- 12 predicates → 12 Examples
- 5 advanced (Clone, Format, As, IsKernelError, third-party) → 8 Examples
- 10 business scenarios → 10 Examples in example_business_test.go

Total: 11 + 14 + 16 + 12 + 8 = 61 in example_test.go + 10 in example_business_test.go = 71 Examples

Public API surface: ~50
Coverage ratio: 61/50 = 122% (some APIs have multiple Examples)

**Result: errorx is doc-complete on Dimension 3.**
