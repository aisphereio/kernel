# Doc Templates for Kernel Modules

Canonical templates established by `errorx/`. Copy these patterns when
generating docs for new modules. **Do not invent new patterns** — consistency
across modules is the whole point.

## Table of contents

1. [README.md template](#readmemd-template)
2. [doc.go template](#docgo-template)
3. [example_test.go template](#example_testgo-template)
4. [example_business_test.go template](#example_business_testgo-template)
5. [docs/ai/<module>.md template](#docsaimodulemd-template)
6. [CLAUDE.md section template](#claudemd-section-template)
7. [.cursor/rules/<module>.mdc template](#cursorrulesmodulemdc-template)
8. [.github/copilot-instructions.md section template](#githubcopilot-instructionsmd-section-template)
9. [AGENTS.md section template](#agentsmd-section-template)
10. [examples/<module>-basic/ template](#examplesmodule-basic-template)
11. [examples/<module>-http/ template](#examplesmodule-http-template)
12. [docs/contracts/<module>.md template](#docscontractsmodulemd-template)
13. [docs/process/<module>-acceptance-checklist.md template](#docsprocessmodule-acceptance-checklistmd-template)
14. [scripts/check-<module>-usage.sh template](#scriptscheck-moduleusagesh-template)

---

## README.md template

```markdown
# <module>

`<module>` 是 Aisphere Kernel 的 <one-line purpose>。它是 Kernel 中**唯一的**
<domain> 包，完全替代旧的 <if applicable>。

> **新手上路**：只看本文件即可上手。需要深度细节时再翻其他文档（见末尾"文档地图"）。

---

## 1. 为什么需要 <module>

<2-3 paragraphs explaining the problem this module solves and why it exists.
Reference the old approach it replaces, if any.>

<module> 的核心契约：**<one-sentence statement of the core contract>**

```text
<module> (只定义 <what>)
  ↓ 稳定契约 (<interface names>)
logx     → <function>(err)         <what logx extracts>
httpx    → <function>(err)         <what httpx does>
grpcx    → <function>(err)         <what grpcx does>
auditx   → <function>(err)         <what auditx records>
metricsx → <function>(err)         <what metricsx emits>
workerx  → <function>(err)         <what workerx decides>
```

<module> 本身**不<list of things it doesn't do>**。它只<what it does>，其他模块只消费。

---

## 2. 30 秒上手

```go
package service

import "github.com/aisphereio/kernel/<module>"

func <Operation>(ctx context.Context, id string) (*<Type>, error) {
    <3-5 lines showing the most common usage pattern>
}
```

<Show the expected output / behavior in 1-2 lines below the code block.>

---

## 3. 构造器速查

| 场景 | 构造器 | <property1> | <property2> | <property3> |
|---|---|---:|---|---:|
| <scenario 1> | `<Constructor1>` | <value> | <value> | <value> |
| <scenario 2> | `<Constructor2>` | <value> | <value> | <value> |
| ... | ... | ... | ... | ... |

```go
<Constructor1>("CODE_NAME", "message")
<Constructor2>("CODE_NAME", "message")
// ...
```

<Note any aliases here, e.g. "Internal and InternalServer are aliases".>

---

## 4. Option 列表

构造时按需附加：

```go
<module>.WithMessage(msg)                  // 覆盖默认 message
<module>.With<Property>(value)             // 覆盖默认 <property>
<module>.WithMetadata("key", "value")      // 内部 metadata
<module>.WithPublicMetadata("key", "value")  // 公开 metadata
// ...
```

---

## 5. 包装底层错误

<If module wraps errors (like errorx), include this section. Otherwise skip.>

`Wrap` 保留底层 cause，支持 `errors.Is` / `errors.As`：

```go
<example showing Wrap with cause preservation>
```

`Wrap(nil, ...)` 返回 `nil`，调用方无需 nil 检查。

---

## 6. 第三方兼容

<If module integrates with foreign types, include this section. Otherwise skip.>

<module> 通过 `errors.As` 识别第三方错误，**不 import 第三方包**。识别的接口：

```go
<list of interfaces recognized via errors.As>
```

---

## 7. 检查辅助函数

消费方应使用这些函数提取语义，**不要直接类型断言**：

```go
<module>.<Inspect1>(err)            // 提取 <what>
<module>.<Inspect2>(err)            // 提取 <what>
// ...
```

便捷谓词：

```go
<module>.Is<Predicate1>(err)
<module>.Is<Predicate2>(err)
// ...
```

---

## 8. <Module-specific section>

<Add 1-3 sections for module-specific concepts. For errorx this is "Metadata
安全规则". For logx it might be "日志级别" and "结构化字段". For httpx it
might be "中间件链" and "路由分组".>

---

## 9. <Naming / format rules>

<If module has naming rules (like errorx has error code format), include here.>

格式：`{DOMAIN}_{RESOURCE}_{REASON}`，全大写蛇形。

```text
✅ <good example 1>
✅ <good example 2>

❌ <bad example 1>  （<reason>）
❌ <bad example 2>  （<reason>）
```

---

## 10-14. <Additional sections as needed>

<Add sections for: retryable, debugging, testing, etc. — whatever the module
needs. errorx has sections 10-14 covering retryable, debugging %+v, testing,
forbidden patterns, and doc map.>

---

## N. 文档地图

<module> 的文档分为四类，按需查阅：

```text
快速上手
├── 本文件 (<module>/README.md)              ← 你正在看的，单一入口
├── <module>/doc.go                          ← go doc 输出源
└── <module>/example_test.go                 ← Go 标准示例

深度规范（架构师/PR review 时看）
├── docs/design/<module>.md                  ← 设计规范
└── docs/contracts/<module>.md               ← 不可破坏契约

AI 编码指南（AI 写业务代码时看）
├── docs/ai/<module>.md                      ← 合并版 AI 指南
└── AGENTS.md                                ← 项目级 AI 规则

验收与运维（CI/CD 时看）
├── docs/process/<module>-acceptance-checklist.md
└── docs/process/<module>-test-report.md

可运行示例
├── examples/<module>-basic/                 ← 最小示例
└── examples/<module>-http/                  ← HTTP handler 示例
```

**优先级**：日常开发只看本 README + `docs/ai/<module>.md` 即可。

---

## N+1. Examples 索引（按场景查找）

<This section is critical for AI tools. List every ExampleXxx function with
its scenario. See errorx/README.md section 16 for the canonical format.>

### 构造器示例（N 个）

| 构造器 | Example 函数 | 何时用 |
|---|---|---|
| `<Constructor1>` | `Example<Constructor1>` | <when> |
| ... | ... | ... |

### Option 示例（N 个）

### Inspect 示例（N 个）

### 谓词示例（N 个）

### 业务场景示例（10 个）

### 高级示例

### 可运行示例

<See "Examples 索引" section in errorx/README.md for the full table format.>

---

## N+2. 设计哲学一句话

> <one-sentence summary of the module's design principle>
```

---

## doc.go template

```go
// Package <module> defines Aisphere Kernel's standard <domain> semantics.
//
// <module> is the ONLY <domain> package in Kernel. It replaces the
// old <predecessor if any> package.
//
// # Design principle
//
// <module> only DEFINES <what>. It does NOT <list of things it doesn't do>.
// Other Kernel modules (<list>) CONSUME <module> through stable inspect
// helpers such as [<Func1>], [<Func2>].
//
// <module> depends only on the Go standard library.
//
// # 30-second quickstart
//
//	func <Operation>(ctx context.Context, id string) (*<Type>, error) {
//	    <3-5 lines showing the most common usage pattern>
//	}
//
// # Constructors
//
// Use semantic constructors instead of manually setting <properties>:
//
//	<module>.<Constructor1>("CODE", "message")   // <HTTP/status>
//	<module>.<Constructor2>("CODE", "message")   // <HTTP/status>
//	// ...
//
// # Options
//
// Append metadata, cause, etc. via options:
//
//	<module>.<Function>(...,
//	    <module>.With<Option1>(...),
//	    <module>.With<Option2>(...),
//	)
//
// # Inspect helpers
//
// Do NOT type-assert on *<Type>. Use inspect helpers which are nil-safe:
//
//	<module>.<Inspect1>(err)   // <what it returns>
//	<module>.<Inspect2>(err)   // <what it returns>
//
// # <Module-specific rules>
//
// <If module has naming rules or forbidden patterns, document here.>
//
// Format: {DOMAIN}_{RESOURCE}_{REASON}, uppercase snake_case.
//
//	<good example>
//	<bad example>
//
// # Forbidden in business code
//
// <list of forbidden patterns, like errorx forbids errors.New>
//
// # Debugging with %+v
//
// <If applicable, like errorx Format() method.>
//
// # Further reading
//
// See <module>/README.md for the single-source-of-truth user guide, and
// docs/ai/<module>.md for the AI coding recipe.
package <module>
```

---

## example_test.go template

```go
package <module>_test

import (
    "errors"
    "fmt"

    "github.com/aisphereio/kernel/<module>"
)

// ============================================================================
// CONSTRUCTOR EXAMPLES (one per public constructor)
// Each ExampleXxx is shown by `go doc <module>.Xxx`.
// ============================================================================

func Example<Constructor1>() {
    err := <module>.<Constructor1>("CODE_NAME", "message")
    fmt.Println(<module>.<Inspect1>(err))
    fmt.Println(<module>.<Inspect2>(err))
    // Output:
    // CODE_NAME
    // <expected value>
}

func Example<Constructor2>() {
    // ... same pattern
}

// ============================================================================
// OPTION EXAMPLES (one per public Option)
// ============================================================================

func ExampleWith<Option1>() {
    err := <module>.<Constructor>("CODE", "msg",
        <module>.With<Option1>(value),
    )
    fmt.Println(<module>.<Inspect>(err))
    // Output: <expected>
}

// ============================================================================
// INSPECT EXAMPLES (one per public inspect function)
// ============================================================================

func Example<Inspect1>() {
    err := <module>.<Constructor>("CODE", "msg")
    fmt.Println(<module>.<Inspect1>(err))
    // Output: <expected>
}

// Include nil safety examples:
func Example<Inspect1>_nil() {
    fmt.Println(<module>.<Inspect1>(nil))
    // Output: <expected for nil>
}

// ============================================================================
// PREDICATE EXAMPLES (one per public predicate)
// ============================================================================

func ExampleIs<Predicate1>() {
    err := <module>.<Constructor>("CODE", "msg")
    fmt.Println(<module>.Is<Predicate1>(err))
    // Output: true
}

// ============================================================================
// ADVANCED EXAMPLES
// ============================================================================

func Example<Error>_Clone() {
    // Deep copy with override
    original := <module>.<Constructor>("CODE", "msg")
    clone := original.Clone()
    // ...
}

func Example<Error>_Format() {
    // %+v for debugging
    err := <module>.<Constructor>("CODE", "msg")
    fmt.Println(err.Error())
    // Output: msg
}

// ============================================================================
// THIRD-PARTY COMPATIBILITY (if applicable)
// ============================================================================

func ExampleFrom_foreignError() {
    // <module> recognizes foreign errors via <method>
    foreignErr := <foreign type>{...}
    ke := <module>.From(foreignErr)
    fmt.Println(ke.<Property>())
    // Output: <expected>
}
```

**Key rules**:
- Every Example MUST have `// Output:` comment
- Example function name = `Example` + PublicAPI name (e.g. `ExampleNotFound`)
- For nil safety, use `Example<Func>_nil` suffix
- For foreign compat, use `ExampleFrom_foreign<type>` suffix
- Group Examples by category with comment headers

---

## example_business_test.go template

```go
package <module>_test

import (
    "context"
    "errors"
    "fmt"

    "github.com/aisphereio/kernel/<module>"
)

// This file demonstrates COMPLETE business scenarios showing how <module>
// fits into handler → service → repository flow. AI tools should copy these
// patterns when generating new business code.

// ============================================================================
// SCENARIO 1: Repository layer — convert DB errors to <module> errors
// ============================================================================

func ExampleBusiness_repositoryLayer() {
    repo := &fakeRepo{}
    _, err := repo.Find(context.Background(), "missing")
    fmt.Println(<module>.<Inspect>(err))
    // Output: <expected>
}

// ============================================================================
// SCENARIO 2: Service layer — validation + authz + business rules
// ============================================================================

func ExampleBusiness_serviceLayer() {
    svc := &fakeService{}
    _, err := svc.Create(context.Background(), "")
    fmt.Println(<module>.<Inspect>(err))
    // Output: <expected>
}

// ============================================================================
// SCENARIO 3: Upstream dependency failure
// ============================================================================

func ExampleBusiness_upstreamTimeout() {
    err := callUpstream(context.Background())
    fmt.Println(<module>.<Inspect>(err))
    fmt.Println(<module>.<Inspect2>(err))
    // Output:
    // <expected1>
    // <expected2>
}

// ============================================================================
// SCENARIO 4: Authz denied
// ============================================================================

// ============================================================================
// SCENARIO 5: Worker retry decision
// ============================================================================

// ============================================================================
// SCENARIO 6: HTTP response shape
// ============================================================================

// ============================================================================
// SCENARIO 7: Audit record
// ============================================================================

// ============================================================================
// SCENARIO 8: Log entry (with redaction)
// ============================================================================

// ============================================================================
// SCENARIO 9: Metrics labels (low cardinality)
// ============================================================================

// ============================================================================
// SCENARIO 10: Multi-layer wrap (preserve chain)
// ============================================================================

// ============================================================================
// FAKE IMPLEMENTATIONS (for example self-containment)
// ============================================================================

type fakeRepo struct{}
type fakeService struct{}

func (r *fakeRepo) Find(...) { ... }
func (s *fakeService) Create(...) { ... }
```

**The 10 standard scenarios** (adapt to module):
1. Repository layer — convert DB/storage errors
2. Service layer — validation + business rules + conflict
3. Upstream dependency failure
4. Authz denied
5. Worker retry decision
6. HTTP response shape
7. Audit record
8. Log entry (with redaction)
9. Metrics labels (low cardinality)
10. Multi-layer wrap (preserve chain)

---

## docs/ai/<module>.md template

```markdown
# <module> — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的 <domain> 处理规范。**只看本文件即可写对所有场景。**

---

## 0. 一句话规则

> 业务代码（handler/service/repository）<action> 时，**必须**使用 `github.com/aisphereio/kernel/<module>`。
> **禁止**使用 <forbidden alternatives>。

---

## 1. 速查：什么场景用什么构造器

| 业务场景 | 构造器 | <property> | 何时使用 |
|---|---|---:|---|
| <scenario 1> | `<Constructor1>` | <value> | <when> |
| ... | ... | ... | ... |

---

## 2. 标准食谱（10 个场景，复制即用）

### 2.1 <scenario 1>

```go
<code block>
```

### 2.2 <scenario 2>

```go
<code block>
```

<... through 2.10>

---

## 3. <Naming / format rules>

格式：`{DOMAIN}_{RESOURCE}_{REASON}`，全大写蛇形。

<good and bad examples>

---

## 4. 禁止模式

### 4.1 业务代码禁止

```go
// ❌ <forbidden pattern 1>
<code>

// ❌ <forbidden pattern 2>
<code>
```

### 4.2 替代写法

```go
// ✅ <correct pattern 1>
<code>
```

### 4.3 允许的例外

测试代码（`*_test.go`）可以 <exceptions>.

---

## 5. <Module-specific rules>

<e.g. errorx has "Metadata 安全规则" here>

---

## 6. 消费方模式

<for logx/httpx/auditx/metricsx/workerx consumers>

```go
// ✅ 正确
<code using inspect helpers>

// ❌ 错误：直接类型断言
<code with type assertion>
```

---

## 7. 调试技巧

<debugging tips, %+v format, etc.>

---

## 8. 完整 handler 示例

```go
<full handler + service + repo example showing the module in context>
```

---

## 9. 验收清单（写完 <module> 代码后自检）

- [ ] <checklist item 1>
- [ ] <checklist item 2>
- ...

---

## 10. 相关文档

- `<module>/README.md` — 单一入口用户指南
- `<module>/doc.go` — `go doc <module>` 输出源
- `<module>/example_test.go` — 所有构造器的 Go 标准 Example
- `docs/design/<module>.md` — 完整设计规范（深度参考）
- `docs/contracts/<module>.md` — 不可破坏契约
- `docs/process/<module>-acceptance-checklist.md` — CI 验收清单
- `examples/<module>-basic/` — 最小可运行示例
- `examples/<module>-http/` — HTTP handler 完整示例
```

---

## CLAUDE.md section template

Add a section like this to CLAUDE.md for each module:

```markdown
### N. <Module> — use <module>, never <forbidden>

In <code paths> code:

```go
// ❌ FORBIDDEN
<forbidden code>

// ✅ REQUIRED
<correct code using module>
```

<Constructor/function> cheatsheet:

| Scenario | Function | <property> |
|---|---|---:|
| <scenario> | `<function>` | <value> |
| ... | ... | ... |

<Module-specific format rules if any>
```

Also add module Examples to the "Examples — find by scenario" table:

```markdown
### <Module> examples (N)

| Scenario | Example function | File |
|---|---|---|
| <scenario> | `Example<Function>` | `<module>/example_test.go` |
| ... | ... | ... |
```

---

## .cursor/rules/<module>.mdc template

```yaml
---
description: Aisphere Kernel <module> enforcement — ensures AI uses <module> instead of <forbidden>
globs:
  - "**/handler/**/*.go"
  - "**/service/**/*.go"
  - "**/repository/**/*.go"
  # ... adapt to module's use sites
alwaysApply: true  # true for foundational modules (errorx, logx)
---

# <module> enforcement

You are working inside **Aisphere Kernel**. Business code MUST use `github.com/aisphereio/kernel/<module>` for <domain>.

## Forbidden patterns (will fail CI)

In <code paths> Go files (NOT in `*_test.go`):

```go
// ❌ FORBIDDEN — use <module> instead
<forbidden code>
```

## Required patterns

```go
// ✅ <scenario 1>
<correct code>

// ✅ <scenario 2>
<correct code>
```

## Constructor cheatsheet

| Scenario | Constructor | <property> |
|---|---|---:|
| ... | ... | ... |

## <Module-specific rules>

<e.g. naming format, metadata safety, etc.>

## 📚 Examples — find by scenario BEFORE writing code

### Quick lookup by scenario

| Your scenario | Example function | File |
|---|---|---|
| <scenario> | `Example<Function>` | `<module>/example_test.go` |
| ... | ... | ... |

### Business scenario examples

| Scenario | Example | Layer |
|---|---|---|
| Repository <module> usage | `ExampleBusiness_repositoryLayer` | repository |
| ... | ... | ... |

All business examples are in `<module>/example_business_test.go`.

## Reading order before writing code

1. **Look up Example** in the tables above (or run `go doc <module>.<Function>`)
2. `<module>/README.md` — single entry guide
3. `docs/ai/<module>.md` — 10 scenarios + complete handler example
4. `<module>/example_business_test.go` — copy entire business scenario

## Pre-commit checks

```bash
./scripts/check-<module>-usage.sh    # grep for forbidden patterns
golangci-lint run                   # depguard enforces import rules
go test ./...                       # includes all Example tests
```
```

---

## .github/copilot-instructions.md section template

Add a section like this for each module:

```markdown
### N. <Module> — use <module>

In <code paths> code:

```go
// ❌ FORBIDDEN
<forbidden>

// ✅ REQUIRED
<correct>
```

Constructor cheatsheet:

- `<function>` — <property>, <when>
- ...

<Module-specific rules>

## 📚 Examples — find by scenario BEFORE writing code

### Quick lookup

| Scenario | Example | File |
|---|---|---|
| <scenario> | `Example<Function>` | `<module>/example_test.go` |
| ... | ... | ... |

### Complete business scenarios (in `<module>/example_business_test.go`)

| Scenario | Example |
|---|---|
| Repository layer | `ExampleBusiness_repositoryLayer` |
| ... | ... |
```

---

## AGENTS.md section template

Add to the hard rules section:

```markdown
### Rule N: <Module> — use <module>, never <forbidden>

In <code paths> code (NOT in `*_test.go`):

```go
// ❌ FORBIDDEN
<forbidden>

// ✅ REQUIRED
<correct>
```

<Constructor cheatsheet table>
```

Add to the "Examples — find by scenario" section:

```markdown
### <Module> examples

#### Quick lookup: "I want to..."

| Your scenario | Example function | File |
|---|---|---|
| <scenario> | `Example<Function>` | `<module>/example_test.go` |
| ... | ... | ... |

#### Business scenario examples

| Scenario | Example | Layer |
|---|---|---|
| Repository layer | `ExampleBusiness_repositoryLayer` | repository |
| ... | ... | ... |
```

---

## examples/<module>-basic/ template

`main.go`:

```go
// Package main demonstrates <module> basic usage.
//
// Run:
//   go run ./examples/<module>-basic
package main

import (
    "errors"
    "fmt"

    "github.com/aisphereio/kernel/<module>"
)

func main() {
    cause := errors.New("<underlying error>")
    err := <module>.<Constructor>("CODE_NAME", "message",
        <module>.With<Option1>(...),
        <module>.With<Option2>(...),
    )

    fmt.Println("<property>:", <module>.<Inspect1>(err))
    fmt.Println("<property>:", <module>.<Inspect2>(err))
}
```

`README.md`:

```markdown
# <module> basic example

最小可运行示例，展示 <module> 的核心用法。

## 运行

```bash
go run ./examples/<module>-basic
```

## 预期输出

```text
<expected output>
```

## 这个示例展示了什么

<3-5 bullet points explaining what the example demonstrates>

## 下一步

如果要看 HTTP handler 完整示例，跑 `examples/<module>-http`：

```bash
go run ./examples/<module>-http
```

## 相关文档

- `<module>/README.md` — 完整用户指南
- `docs/ai/<module>.md` — AI 编码食谱
```

---

## examples/<module>-http/ template

`main.go`:

```go
// Package main demonstrates a complete HTTP handler using <module>.
//
// Run:
//   go run ./examples/<module>-http
//
// Then in another terminal:
//   curl -i http://localhost:18080/<resource>/<scenario>
package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/aisphereio/kernel/<module>"
)

// Domain error codes
const (
    Err<Resource>NotFound     <module>.Code = "DOMAIN_RESOURCE_NOT_FOUND"
    Err<Resource>NameRequired <module>.Code = "DOMAIN_RESOURCE_NAME_REQUIRED"
    // ...
)

// Handler
type <Resource>Handler struct{}

func (h *<Resource>Handler) Get(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Query().Get("id")
    if id == "" {
        writeError(w, <module>.BadRequest(Err<Resource>NameRequired, "id required"))
        return
    }
    // ... business logic with <module> error returns
}

// writeError converts <module> error to HTTP response
func writeError(w http.ResponseWriter, err error) {
    resp := map[string]any{
        "code":    <module>.<Inspect>(err),
        "message": <module>.<Inspect>(err),
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(<module>.<Inspect>(err))
    _ = json.NewEncoder(w).Encode(resp)
}

func main() {
    mux := http.NewServeMux()
    h := &<Resource>Handler{}
    mux.HandleFunc("/<resource>", h.Get)

    srv := &http.Server{Addr: ":18080", Handler: mux}
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    go func() {
        fmt.Printf("listening on :18080\n")
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            panic(err)
        }
    }()
    <-ctx.Done()
    _ = srv.Shutdown(context.Background())
}
```

`README.md`:

```markdown
# <module> HTTP example

完整 HTTP handler 示例，展示 <module> 在真实业务场景下的端到端用法。

## 运行

```bash
go run ./examples/<module>-http
```

## 测试不同错误场景

```bash
# 200 OK
curl -i 'http://localhost:18080/<resource>?id=<valid>'

# 400 Bad Request
curl -i 'http://localhost:18080/<resource>?id='

# 404 Not Found
curl -i 'http://localhost:18080/<resource>?id=missing'

# ... more scenarios
```

## 场景 → <module> 构造器 → HTTP 响应对应表

| # | curl 场景 | <module> 构造器 | HTTP | error_code | 对应 Example |
|---|---|---|---:|---|---|
| 1 | `?id=<valid>` | （无错误） | 200 | — | — |
| 2 | `?id=` | `<module>.BadRequest` | 400 | `DOMAIN_<RESOURCE>_NAME_REQUIRED` | `ExampleBadRequest` |
| 3 | `?id=missing` | `<module>.NotFound` | 404 | `DOMAIN_<RESOURCE>_NOT_FOUND` | `ExampleNotFound` |
| ... | ... | ... | ... | ... | ... |

## 学到什么

<5-7 bullet points explaining what the example demonstrates>

## 相关文档与示例

| 想学什么 | 看哪里 |
|---|---|
| <module> 完整指南 | `<module>/README.md` |
| AI 编码食谱 | `docs/ai/<module>.md` |
| 所有构造器 Example | `<module>/example_test.go` |
| 完整业务场景 Example | `<module>/example_business_test.go` |
```

---

## docs/contracts/<module>.md template

```markdown
# <module> Contract

模块：`github.com/aisphereio/kernel/<module>`
验收等级目标：L3 可生产试用
版本：v0.1.0-alpha.1

## 1. Contract 目标

<1-2 paragraphs explaining what the contract guarantees.>

只要以下 contract 不被破坏，内部实现可以重构。

## 2. 不可破坏行为

### 2.1 基础映射

| 输入 | <Inspect1> | <Inspect2> | ... |
|---|---|---|---|
| `nil` | <value> | <value> | ... |
| unknown | <value> | <value> | ... |

### 2.2 构造函数

| 构造函数 | 默认 <prop1> | 默认 <prop2> | ... |
|---|---|---|---|
| `<Constructor1>` | <value> | <value> | ... |
| ... | ... | ... | ... |

### 2.3 错误链 / 关系保留

- <list of chain/relationship behaviors that must hold>

### 2.4 Inspect 安全性

<list of inspect functions that must be nil-safe and not panic>

### 2.5 第三方兼容

<list of interfaces recognized via errors.As>

### 2.6 <Module-specific safety>

<e.g. errorx has "Metadata 安全" and "Metrics 低基数" sections>

## 3. Breaking Change 判定

以下变更必须作为 breaking change 记录：

- <list of changes that break the contract>

## 4. 验收命令

```bash
go test ./<module> -v
go test ./<module> -race
go test ./<module> -cover
go test ./...
go vet ./...
go test ./<module> -bench=.
```

可选 fuzz：

```bash
go test ./<module> -run=^$ -fuzz=Fuzz<Function> -fuzztime=30s
```
```

---

## docs/process/<module>-acceptance-checklist.md template

```markdown
# <module> Acceptance Checklist

## Static checks

- [ ] `grep -R "github.com/aisphereio/kernel/<predecessor>" --include="*.go" .` returns no result (if replacing old package)
- [ ] <list of static checks>

## Unit checks

Run:

```bash
go test ./<module> -v
go test ./<module> -race
go test ./<module> -cover
```

Expected coverage areas:

- <list of coverage areas>

## Integration checks

Run on a full local machine:

```bash
go test ./...
go vet ./...
go run ./examples/<module>-basic
go run ./examples/<module>-http
```

Expected scenarios:

1. <scenario 1>
2. <scenario 2>
...

## Windows commands

<Windows-specific commands if applicable>
```

---

## scripts/check-<module>-usage.sh template

```bash
#!/usr/bin/env bash
#
# check-<module>-usage.sh — verify business code uses <module>, not <forbidden>.
#
# Usage:
#   ./scripts/check-<module>-usage.sh           # check all business .go files
#   ./scripts/check-<module>-usage.sh --verbose # print every checked file
#
# Exit codes:
#   0  no violations
#   1  violations found (CI should fail)
#   2  script error

set -euo pipefail

MODE="${1:-check}"
VERBOSE=false
case "$MODE" in
  --verbose) VERBOSE=true ;;
  --check|check) ;;
  *) echo "usage: $0 [--check|--verbose]"; exit 2 ;;
esac

# Colors
if [ -t 1 ]; then
  RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'; NC='\033[0m'
else
  RED=''; YELLOW=''; GREEN=''; NC=''
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Find business .go files (NOT tests, NOT module internals, NOT examples)
mapfile -t FILES < <(
  find . \( -name '*_test.go' -prune -o \
            -name '*.pb.go' -prune -o \
            -path './<module>/*' -prune -o \
            -path './examples/*' -prune -o \
            -path './contrib/*' -prune -o \
            -type d -name vendor -prune \) -o \
    \( -path './handler/*' -o -path './service/*' -o -path './repository/*' \
       -o -path './internal/handler/*' -o -path './internal/service/*' \) \
    -name '*.go' -print
)

if [ "${#FILES[@]}" -eq 0 ]; then
  echo "${GREEN}✓${NC} no business .go files found"
  exit 0
fi

VIOLATIONS=0
REPORT=""

check_pattern() {
  local label="$1"
  local pattern="$2"
  local suggestion="$3"
  for file in "${FILES[@]}"; do
    while IFS= read -r line; do
      [ -n "$line" ] || continue
      VIOLATIONS=$((VIOLATIONS + 1))
      REPORT+="${RED}✗${NC} ${label}
  file: ${YELLOW}${file}${NC}:${line%%:*}
  code: $(echo "$line" | cut -d: -f2-)
  fix:  ${suggestion}
"
    done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)
  done
}

# === Rules ===
check_pattern "<forbidden pattern 1>" '<regex>' '<suggestion>'
check_pattern "<forbidden pattern 2>" '<regex>' '<suggestion>'
# ... add more rules

if [ "$VIOLATIONS" -eq 0 ]; then
  echo "${GREEN}✓${NC} <module> usage OK — ${#FILES[@]} business files checked"
  exit 0
fi

echo "${RED}✗${NC} <module> usage check failed — $VIOLATIONS violation(s)"
echo "$REPORT"
echo "Read docs/ai/<module>.md for the complete AI coding recipe."
exit 1
```

Make executable: `chmod +x scripts/check-<module>-usage.sh`
