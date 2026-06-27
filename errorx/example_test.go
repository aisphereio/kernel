package errorx_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/aisphereio/kernel/errorx"
)

// ============================================================================
// CONSTRUCTOR EXAMPLES (11 constructors)
// Each ExampleXxx is shown by `go doc errorx.Xxx`.
// ============================================================================

func ExampleBadRequest() {
	err := errorx.BadRequest("REQUEST_VALIDATE_FAILED", "请求参数校验失败")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.GRPCCodeOf(err))
	fmt.Println(errorx.RetryableOf(err))
	// Output:
	// REQUEST_VALIDATE_FAILED
	// 400
	// InvalidArgument
	// false
}

func ExampleUnauthorized() {
	err := errorx.Unauthorized("AUTH_TOKEN_MISSING", "缺少授权令牌")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.CategoryOf(err))
	// Output:
	// 401
	// auth
}

func ExampleForbidden() {
	err := errorx.Forbidden("IAM_PERMISSION_DENIED", "权限被拒绝",
		errorx.WithMetadata("subject_id", "user_123"),
		errorx.WithPublicMetadata("resource", "skill"),
	)
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.PublicMetadataOf(err))
	// Output:
	// 403
	// map[resource:skill]
}

func ExampleNotFound() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.RetryableOf(err))
	// Output:
	// AIHUB_SKILL_NOT_FOUND
	// 404
	// false
}

func ExampleConflict() {
	err := errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.CategoryOf(err))
	// Output:
	// 409
	// conflict
}

func ExampleRequestTimeout() {
	err := errorx.RequestTimeout("HTTP_REQUEST_TIMEOUT", "请求超时")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.GRPCCodeOf(err))
	// Output:
	// 408
	// DeadlineExceeded
}

func ExampleTooManyRequests() {
	err := errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "请求过于频繁")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.RetryableOf(err))
	// Output:
	// 429
	// true
}

func ExampleClientClosed() {
	err := errorx.ClientClosed("CLIENT_CLOSED", "客户端断开连接")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.GRPCCodeOf(err))
	fmt.Println(errorx.SeverityOf(err))
	// Output:
	// 499
	// Canceled
	// debug
}

func ExampleInternal() {
	err := errorx.Internal("AIHUB_INTERNAL_ERROR", "服务器内部错误")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.CategoryOf(err))
	fmt.Println(errorx.SeverityOf(err))
	// Output:
	// 500
	// internal
	// error
}

func ExampleUnavailable() {
	err := errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "模型上游不可用")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.RetryableOf(err))
	// Output:
	// 503
	// true
}

func ExampleTimeout() {
	err := errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型上游超时")
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.RetryableOf(err))
	fmt.Println(errorx.CategoryOf(err))
	// Output:
	// 504
	// true
	// dependency
}

// ============================================================================
// NEW / Wrap / From (3 core constructors)
// ============================================================================

func ExampleNew() {
	// New creates a canonical Error with default mappings derived from code.
	err := errorx.New("AIHUB_CUSTOM_ERROR",
		errorx.WithMessage("自定义错误"),
		errorx.WithHTTPStatus(422),
	)
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.MessageOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	// Output:
	// AIHUB_CUSTOM_ERROR
	// 自定义错误
	// 422
}

func ExampleWrap() {
	cause := errors.New("pq: connection refused")
	err := errorx.Wrap(cause, "AIHUB_SKILL_CREATE_FAILED",
		errorx.WithMessage("创建技能失败"),
		errorx.WithRetryable(true),
	)

	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.MessageOf(err))
	fmt.Println(errorx.RetryableOf(err))
	fmt.Println(errors.Is(err, cause))
	// Output:
	// AIHUB_SKILL_CREATE_FAILED
	// 创建技能失败
	// true
	// true
}

func ExampleWrap_nil() {
	// Wrap(nil, ...) returns nil — safe to chain without nil check.
	err := errorx.Wrap(nil, "AIHUB_SKILL_CREATE_FAILED")
	fmt.Println(err == nil)
	// Output: true
}

func ExampleFrom() {
	// From converts a foreign error to *errorx.Error.
	cause := errors.New("something failed")
	ke := errorx.From(cause)
	fmt.Println(ke.Code())
	fmt.Println(ke.HTTPStatus())
	fmt.Println(ke.Retryable())
	// Output:
	// INTERNAL_ERROR
	// 500
	// false
}

func ExampleFrom_withOptions() {
	// From with options to override defaults.
	cause := errors.New("upstream timeout")
	ke := errorx.From(cause,
		errorx.WithHTTPStatus(504),
		errorx.WithRetryable(true),
		errorx.WithMetadata("upstream", "model-svc"),
	)
	fmt.Println(ke.HTTPStatus())
	fmt.Println(ke.Retryable())
	fmt.Println(ke.Metadata()["upstream"])
	// Output:
	// 504
	// true
	// model-svc
}

// ============================================================================
// OPTION EXAMPLES (12 options)
// ============================================================================

func ExampleWithMessage() {
	err := errorx.New("AIHUB_SKILL_NOT_FOUND",
		errorx.WithMessage("技能不存在"),
	)
	fmt.Println(err.Error())
	// Output: 技能不存在
}

func ExampleWithHTTPStatus() {
	// Override default HTTP status (use sparingly — prefer semantic constructors).
	err := errorx.New("AIHUB_PAYMENT_REQUIRED",
		errorx.WithHTTPStatus(402),
	)
	fmt.Println(errorx.HTTPStatusOf(err))
	// Output: 402
}

func ExampleWithGRPCCode() {
	err := errorx.New("AIHUB_CUSTOM",
		errorx.WithGRPCCode("FailedPrecondition"),
	)
	fmt.Println(errorx.GRPCCodeOf(err))
	// Output: FailedPrecondition
}

func ExampleWithRetryable() {
	// Override default retryable. Use when business knows better than defaults.
	err := errorx.Internal("AIHUB_DB_TRANSACTION_FAILED", "数据库事务失败",
		errorx.WithRetryable(true),
	)
	fmt.Println(errorx.RetryableOf(err))
	// Output: true
}

func ExampleWithCause() {
	// WithCause preserves the underlying error for errors.Is / errors.As.
	cause := errors.New("redis: connection refused")
	err := errorx.Unavailable("CACHE_UNAVAILABLE", "缓存不可用",
		errorx.WithCause(cause),
	)
	fmt.Println(errors.Is(err, cause))
	// Output: true
}

func ExampleWithMetadata() {
	// Internal metadata — for logs/audit only, NOT returned to clients.
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithMetadata("project_id", "proj_001"),
		errorx.WithMetadata("raw_db_error", "record not found"),
	)
	md := errorx.MetadataOf(err)
	fmt.Println(md["skill_id"])
	fmt.Println(md["project_id"])
	// Output:
	// skill_001
	// proj_001
}

func ExampleWithMetadataMap() {
	// WithMetadataMap adds multiple metadata entries at once.
	err := errorx.Internal("AIHUB_DB_FAILED", "数据库失败",
		errorx.WithMetadataMap(map[string]any{
			"dsn":   "postgres://...",
			"query": "SELECT * FROM skills",
		}),
	)
	md := errorx.MetadataOf(err)
	fmt.Println(md["dsn"])
	// Output: postgres://...
}

func ExampleWithPublicMetadata() {
	// PublicMetadata is safe for HTTP/gRPC responses.
	// Metadata is for internal logs/audit only.
	err := errorx.Forbidden("IAM_PERMISSION_DENIED", "权限被拒绝",
		errorx.WithMetadata("raw_policy", "deny"),      // internal
		errorx.WithPublicMetadata("resource", "skill"), // public
		errorx.WithPublicMetadata("action", "delete"),  // public
	)
	fmt.Println(errorx.PublicMetadataOf(err))
	// Output: map[action:delete resource:skill]
}

func ExampleWithPublicMetadataMap() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithPublicMetadataMap(map[string]any{
			"resource": "skill",
			"name":     "demo-skill",
		}),
	)
	fmt.Println(errorx.PublicMetadataOf(err)["resource"])
	// Output: skill
}

func ExampleWithRequestID() {
	err := errorx.Internal("AIHUB_FAILED", "失败",
		errorx.WithRequestID("req_abc123"),
	)
	fmt.Println(errorx.RequestIDOf(err))
	// Output: req_abc123
}

func ExampleWithTraceID() {
	err := errorx.Internal("AIHUB_FAILED", "失败",
		errorx.WithTraceID("trace_xyz789"),
	)
	fmt.Println(errorx.TraceIDOf(err))
	// Output: trace_xyz789
}

func ExampleWithCategory() {
	// Override default category (use sparingly — defaults are usually right).
	err := errorx.New("AIHUB_BUSINESS_RULE_VIOLATION",
		errorx.WithCategory(errorx.CategoryValidation),
	)
	fmt.Println(errorx.CategoryOf(err))
	// Output: validation
}

func ExampleWithSeverity() {
	// Override default severity for logx.
	err := errorx.New("AIHUB_DEGRADED",
		errorx.WithMessage("服务降级"),
		errorx.WithSeverity(errorx.SeverityWarn),
	)
	fmt.Println(errorx.SeverityOf(err))
	// Output: warn
}

func ExampleWithStack() {
	// Capture call stack for debugging. Only use for 5xx errors.
	err := errorx.Internal("AIHUB_INTERNAL_FAILED", "内部错误",
		errorx.WithStack(),
	)
	frames := errorx.StackOf(err)
	if len(frames) > 0 {
		fmt.Println("stack captured")
	}
	// Output: stack captured
}

// ============================================================================
// INSPECT EXAMPLES (13 inspect helpers)
// ============================================================================

func ExampleCodeOf() {
	// CodeOf returns the error code. nil → OK, unknown → INTERNAL_ERROR.
	fmt.Println(errorx.CodeOf(nil))
	fmt.Println(errorx.CodeOf(errors.New("unknown")))
	fmt.Println(errorx.CodeOf(errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "x")))
	// Output:
	// OK
	// INTERNAL_ERROR
	// AIHUB_SKILL_NOT_FOUND
}

func ExampleMessageOf() {
	// MessageOf returns the human-readable message.
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	fmt.Println(errorx.MessageOf(err))
	fmt.Println(errorx.MessageOf(nil))
	// Output:
	// 技能不存在
	// success
}

func ExampleHTTPStatusOf() {
	// HTTPStatusOf returns the HTTP status. nil → 200, unknown → 500.
	fmt.Println(errorx.HTTPStatusOf(nil))
	fmt.Println(errorx.HTTPStatusOf(errors.New("x")))
	fmt.Println(errorx.HTTPStatusOf(errorx.NotFound("X", "x")))
	// Output:
	// 200
	// 500
	// 404
}

func ExampleGRPCCodeOf() {
	err := errorx.BadRequest("X", "x")
	fmt.Println(errorx.GRPCCodeOf(err))
	// Output: InvalidArgument
}

func ExampleRetryableOf() {
	fmt.Println(errorx.RetryableOf(errorx.TooManyRequests("X", "x")))
	fmt.Println(errorx.RetryableOf(errorx.BadRequest("X", "x")))
	// Output:
	// true
	// false
}

func ExampleMetadataOf() {
	err := errorx.NotFound("X", "x",
		errorx.WithMetadata("skill_id", "skill_001"),
	)
	fmt.Println(errorx.MetadataOf(err))
	// Output: map[skill_id:skill_001]
}

func ExampleSafeMetadataOf() {
	// Sensitive keys are redacted for safe logging.
	err := errorx.Internal("X", "x",
		errorx.WithMetadata("password", "s3cr3t"),
		errorx.WithMetadata("dsn", "postgres://..."),
	)
	safe := errorx.SafeMetadataOf(err)
	fmt.Println(safe["password"])
	fmt.Println(safe["dsn"])
	// Output:
	// [REDACTED]
	// postgres://...
}

func ExamplePublicMetadataOf() {
	err := errorx.Forbidden("X", "x",
		errorx.WithMetadata("internal", "secret"),
		errorx.WithPublicMetadata("resource", "skill"),
	)
	// PublicMetadataOf only returns explicitly public fields.
	fmt.Println(errorx.PublicMetadataOf(err))
	// Output: map[resource:skill]
}

func ExampleRequestIDOf() {
	err := errorx.Internal("X", "x", errorx.WithRequestID("req_123"))
	fmt.Println(errorx.RequestIDOf(err))
	// Output: req_123
}

func ExampleTraceIDOf() {
	err := errorx.Internal("X", "x", errorx.WithTraceID("trace_abc"))
	fmt.Println(errorx.TraceIDOf(err))
	// Output: trace_abc
}

func ExampleCategoryOf() {
	fmt.Println(errorx.CategoryOf(errorx.NotFound("X", "x")))
	fmt.Println(errorx.CategoryOf(errorx.Forbidden("X", "x")))
	fmt.Println(errorx.CategoryOf(errorx.Internal("X", "x")))
	// Output:
	// not_found
	// permission
	// internal
}

func ExampleSeverityOf() {
	fmt.Println(errorx.SeverityOf(errorx.BadRequest("X", "x")))
	fmt.Println(errorx.SeverityOf(errorx.Internal("X", "x")))
	fmt.Println(errorx.SeverityOf(errorx.ClientClosed("X", "x")))
	// Output:
	// info
	// error
	// debug
}

func ExampleStackOf() {
	err := errorx.Internal("X", "x", errorx.WithStack())
	frames := errorx.StackOf(err)
	fmt.Println(len(frames) > 0)
	// Output: true
}

// ============================================================================
// FIELDS / METRICS LABELS EXAMPLES (for logx/auditx/metricsx consumers)
// ============================================================================

func ExampleFields() {
	// Fields returns logger-neutral fields for logx/auditx.
	err := errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
		errorx.WithMetadata("model", "qwen3"),
		errorx.WithRequestID("req_456"),
	)
	fields := errorx.Fields(err)
	fmt.Println(fields["error_code"])
	fmt.Println(fields["http_status"])
	fmt.Println(fields["retryable"])
	fmt.Println(fields["category"])
	fmt.Println(fields["severity"])
	fmt.Println(fields["request_id"])
	// Output:
	// MODEL_UPSTREAM_TIMEOUT
	// 504
	// true
	// dependency
	// error
	// req_456
}

func ExampleMetricsLabels() {
	// MetricsLabels returns low-cardinality labels for Prometheus.
	// It NEVER includes message, request_id, trace_id, or object IDs.
	err := errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
		errorx.WithMetadata("model", "qwen3"),
		errorx.WithRequestID("req_456"),
	)
	labels := errorx.MetricsLabels(err)
	fmt.Println(labels["error_code"])
	fmt.Println(labels["http_status"])
	fmt.Println(labels["retryable"])
	fmt.Println(labels["category"])
	// Output:
	// MODEL_UPSTREAM_TIMEOUT
	// 504
	// true
	// dependency
}

// ============================================================================
// PREDICATE EXAMPLES (11 predicates)
// ============================================================================

func ExampleIsBadRequest() {
	fmt.Println(errorx.IsBadRequest(errorx.BadRequest("X", "x")))
	fmt.Println(errorx.IsBadRequest(errorx.NotFound("X", "x")))
	// Output:
	// true
	// false
}

func ExampleIsUnauthorized() {
	fmt.Println(errorx.IsUnauthorized(errorx.Unauthorized("X", "x")))
	// Output: true
}

func ExampleIsForbidden() {
	fmt.Println(errorx.IsForbidden(errorx.Forbidden("X", "x")))
	// Output: true
}

func ExampleIsNotFound() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	fmt.Println(errorx.IsNotFound(err))
	fmt.Println(errorx.IsBadRequest(err))
	// Output:
	// true
	// false
}

func ExampleIsConflict() {
	fmt.Println(errorx.IsConflict(errorx.Conflict("X", "x")))
	// Output: true
}

func ExampleIsRequestTimeout() {
	fmt.Println(errorx.IsRequestTimeout(errorx.RequestTimeout("X", "x")))
	// Output: true
}

func ExampleIsTooManyRequests() {
	fmt.Println(errorx.IsTooManyRequests(errorx.TooManyRequests("X", "x")))
	// Output: true
}

func ExampleIsClientClosedRequest() {
	fmt.Println(errorx.IsClientClosedRequest(errorx.ClientClosed("X", "x")))
	// Output: true
}

func ExampleIsInternal() {
	fmt.Println(errorx.IsInternal(errorx.Internal("X", "x")))
	// Output: true
}

func ExampleIsUnavailable() {
	fmt.Println(errorx.IsUnavailable(errorx.Unavailable("X", "x")))
	// Output: true
}

func ExampleIsTimeout() {
	fmt.Println(errorx.IsTimeout(errorx.Timeout("X", "x")))
	// Output: true
}

func ExampleIsCode() {
	// Match by exact error code (most precise predicate).
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	fmt.Println(errorx.IsCode(err, errorx.Code("AIHUB_SKILL_NOT_FOUND")))
	fmt.Println(errorx.IsCode(err, errorx.Code("AIHUB_USER_NOT_FOUND")))
	// Output:
	// true
	// false
}

// ============================================================================
// AS / IsKernelError EXAMPLES
// ============================================================================

func ExampleAs() {
	// As extracts *errorx.Error from an error chain.
	original := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	wrapped := fmt.Errorf("repo: %w", original)

	ke, ok := errorx.As(wrapped)
	fmt.Println(ok)
	fmt.Println(ke.Code())
	// Output:
	// true
	// AIHUB_SKILL_NOT_FOUND
}

func ExampleIsKernelError() {
	// IsKernelError reports whether err is or wraps an *errorx.Error.
	fmt.Println(errorx.IsKernelError(errorx.NotFound("X", "x")))
	fmt.Println(errorx.IsKernelError(errors.New("plain")))
	// Output:
	// true
	// false
}

// ============================================================================
// CLONE EXAMPLE
// ============================================================================

func ExampleError_Clone() {
	// Clone returns a deep copy. Useful when adding metadata without mutating original.
	original := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithMetadata("skill_id", "skill_001"),
	)
	clone := original.Clone()
	clone = errorx.From(clone, errorx.WithMetadata("extra", "value"))

	fmt.Println(errorx.MetadataOf(original)["extra"])
	fmt.Println(errorx.MetadataOf(clone)["extra"])
	// Output:
	// <nil>
	// value
}

// ============================================================================
// FORMAT EXAMPLE (%+v for debugging)
// ============================================================================

func ExampleError_Format() {
	// %+v prints full debug info; %s and Error() print safe message.
	err := errorx.Internal("AIHUB_SKILL_QUERY_FAILED", "查询技能失败",
		errorx.WithCause(errors.New("pq: connection refused")),
		errorx.WithRequestID("req_123"),
	)
	fmt.Println(err.Error())
	// Output: 查询技能失败
}

func ExampleError_Format_verbose() {
	// fmt.Printf("%+v", err) prints all fields for debugging.
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithPublicMetadata("resource", "skill"),
	)
	// Don't assert exact output (cause formatting varies), just show it works.
	output := fmt.Sprintf("%+v", err)
	fmt.Println(len(output) > 0)
	// Output: true
}

// ============================================================================
// THIRD-PARTY ERROR COMPATIBILITY EXAMPLES
// ============================================================================

// gofrStyleError mimics GoFr's error type with StatusCode() method.
type gofrStyleError struct {
	msg    string
	status int
}

func (e gofrStyleError) Error() string   { return e.msg }
func (e gofrStyleError) StatusCode() int { return e.status }

func ExampleFrom_foreignStatusCode() {
	// errorx recognizes GoFr-style errors via StatusCode() method.
	gofrErr := gofrStyleError{msg: "not found", status: 404}
	ke := errorx.From(gofrErr)
	fmt.Println(ke.HTTPStatus())
	fmt.Println(ke.Code())
	// Output:
	// 404
	// NOT_FOUND
}

// httpStatusError mimics Kratos-style errors with HTTPStatus() method.
type httpStatusError struct {
	msg    string
	status int
}

func (e httpStatusError) Error() string   { return e.msg }
func (e httpStatusError) HTTPStatus() int { return e.status }

func ExampleHTTPStatusOf_foreignError() {
	// errorx recognizes HTTPStatus() method via errors.As.
	foreignErr := httpStatusError{msg: "forbidden", status: 403}
	fmt.Println(errorx.HTTPStatusOf(foreignErr))
	fmt.Println(errorx.CodeOf(foreignErr))
	// Output:
	// 403
	// FORBIDDEN
}

// errorCodeStringError mimics errors that expose ErrorCode() string method.
type errorCodeStringError struct {
	code string
	msg  string
}

func (e errorCodeStringError) Error() string     { return e.msg }
func (e errorCodeStringError) ErrorCode() string { return e.code }

func ExampleCodeOf_foreignError() {
	// errorx recognizes ErrorCode() string method.
	foreignErr := errorCodeStringError{code: "CUSTOM_NOT_FOUND", msg: "x"}
	fmt.Println(errorx.CodeOf(foreignErr))
	// Output: CUSTOM_NOT_FOUND
}

func ExampleWrap_seesThroughFmtErrorf() {
	// errorx.Wrap preserves the chain even through fmt.Errorf wrapping.
	inner := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	wrapped := fmt.Errorf("repo layer: %w", inner)

	// All inspect helpers see through the wrapping.
	fmt.Println(errorx.CodeOf(wrapped))
	fmt.Println(errorx.HTTPStatusOf(wrapped))
	fmt.Println(errorx.IsKernelError(wrapped))
	// Output:
	// AIHUB_SKILL_NOT_FOUND
	// 404
	// true
}

// ============================================================================
// NIL SAFETY EXAMPLES
// ============================================================================

func ExampleCodeOf_nil() {
	// CodeOf(nil) returns OK — safe to call on any error.
	fmt.Println(errorx.CodeOf(nil))
	// Output: OK
}

func ExampleHTTPStatusOf_nil() {
	// HTTPStatusOf(nil) returns 200 — safe to call on any error.
	fmt.Println(errorx.HTTPStatusOf(nil))
	// Output: 200
}

func ExampleFields_nil() {
	// Fields(nil) returns valid map with OK status — safe to call on any error.
	fields := errorx.Fields(nil)
	fmt.Println(fields["error_code"])
	fmt.Println(fields["http_status"])
	// Output:
	// OK
	// 200
}

// ============================================================================
// ERROR CODE VALIDATION EXAMPLES
// ============================================================================

func ExampleIsValidCode() {
	fmt.Println(errorx.IsValidCode("AIHUB_SKILL_NOT_FOUND"))
	fmt.Println(errorx.IsValidCode("skillNotFound"))
	fmt.Println(errorx.IsValidCode(""))
	fmt.Println(errorx.IsValidCode("SKILL_123_NOT_FOUND"))
	// Output:
	// true
	// false
	// false
	// true
}

func ExampleNormalizeCode() {
	// NormalizeCode converts empty code to INTERNAL_ERROR.
	fmt.Println(errorx.NormalizeCode(""))
	fmt.Println(errorx.NormalizeCode("AIHUB_CUSTOM"))
	// Output:
	// INTERNAL_ERROR
	// AIHUB_CUSTOM
}

// ============================================================================
// ERROR METHOD EXAMPLES (on *Error receiver)
// ============================================================================

func ExampleError_Code() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "x")
	fmt.Println(err.Code())
	// Output: AIHUB_SKILL_NOT_FOUND
}

func ExampleError_ErrorCode() {
	// ErrorCode() returns string — for adapters that don't use errorx.Code.
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "x")
	fmt.Println(err.ErrorCode())
	// Output: AIHUB_SKILL_NOT_FOUND
}

func ExampleError_Message() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	fmt.Println(err.Message())
	// Output: 技能不存在
}

func ExampleError_Cause() {
	// Cause returns the wrapped underlying error.
	cause := errors.New("db timeout")
	err := errorx.Wrap(cause, "X", errorx.WithMessage("失败"))
	fmt.Println(errorx.CauseOf(err) == cause)
	// Output: true
}

func ExampleError_Unwrap() {
	// Unwrap enables errors.Is / errors.As.
	cause := errors.New("db timeout")
	err := errorx.Wrap(cause, "X", errorx.WithMessage("失败"))
	fmt.Println(errors.Unwrap(err) == cause)
	// Output: true
}

// ============================================================================
// CONTEXT INTEGRATION EXAMPLE (with request_id / trace_id from context)
// ============================================================================

// NOTE: errorx cannot import contextx (cycle). Business code should extract
// request_id / trace_id from context and pass via WithRequestID / WithTraceID.
// This Example shows the typical pattern.

func Example_withContext() {
	// Simulate extracting request_id from context (in real code, use contextx).
	ctx := context.WithValue(context.Background(), ctxKeyRequestID{}, "req_abc")

	// In real code: reqID, _ := contextx.RequestIDFromContext(ctx)
	reqID, _ := ctx.Value(ctxKeyRequestID{}).(string)

	err := errorx.Internal("AIHUB_FAILED", "失败",
		errorx.WithRequestID(reqID),
	)
	fmt.Println(errorx.RequestIDOf(err))
	// Output: req_abc
}

type ctxKeyRequestID struct{}
