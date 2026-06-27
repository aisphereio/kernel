package errorx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

type foreignStatusError struct {
	msg    string
	status int
}

func (e foreignStatusError) Error() string   { return e.msg }
func (e foreignStatusError) StatusCode() int { return e.status }

type foreignHTTPStatusError struct {
	msg    string
	status int
}

func (e foreignHTTPStatusError) Error() string   { return e.msg }
func (e foreignHTTPStatusError) HTTPStatus() int { return e.status }

type foreignCodedError struct{}

func (foreignCodedError) Error() string     { return "foreign coded" }
func (foreignCodedError) ErrorCode() string { return "MODEL_UPSTREAM_TIMEOUT" }
func (foreignCodedError) Message() string   { return "模型服务超时" }
func (foreignCodedError) Retryable() bool   { return true }
func (foreignCodedError) GRPCCode() string  { return errorx.GRPCCodeDeadlineExceeded }
func (foreignCodedError) Metadata() map[string]any {
	return map[string]any{"model": "qwen3", "request_id": "req_meta", "trace_id": "trace_meta"}
}
func (foreignCodedError) PublicMetadata() map[string]any { return map[string]any{"field": "model"} }
func (foreignCodedError) Category() errorx.Category      { return errorx.CategoryDependency }
func (foreignCodedError) Severity() errorx.Severity      { return errorx.SeverityError }

type foreignLogLevelError struct{}

func (foreignLogLevelError) Error() string   { return "warn level" }
func (foreignLogLevelError) StatusCode() int { return 409 }
func (foreignLogLevelError) LogLevel() int   { return 4 }

type foreignMetadataFieldError struct {
	Metadata map[string]any
}

func (foreignMetadataFieldError) Error() string { return "metadata field" }

func TestInspectNilAndUnknown(t *testing.T) {
	t.Parallel()

	assertEqual(t, errorx.CodeOf(nil), errorx.CodeOK)
	assertEqual(t, errorx.MessageOf(nil), "success")
	assertEqual(t, errorx.HTTPStatusOf(nil), 200)
	assertEqual(t, errorx.GRPCCodeOf(nil), errorx.GRPCCodeOK)
	assertEqual(t, errorx.RetryableOf(nil), false)
	assertEqual(t, errorx.CategoryOf(nil), errorx.CategoryOK)
	assertEqual(t, errorx.SeverityOf(nil), errorx.SeverityInfo)

	unknown := errors.New("db password leaked in raw error should be hidden")
	assertEqual(t, errorx.CodeOf(unknown), errorx.CodeInternal)
	assertEqual(t, errorx.MessageOf(unknown), "internal server error")
	assertEqual(t, errorx.HTTPStatusOf(unknown), 500)
	assertEqual(t, errorx.GRPCCodeOf(unknown), errorx.GRPCCodeInternal)
	assertEqual(t, errorx.RetryableOf(unknown), false)
}

func TestInspectWrappedKernelError(t *testing.T) {
	t.Parallel()

	base := errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除权限",
		errorx.WithMetadata("skill_id", "skill_001"),
	)
	wrapped := fmt.Errorf("service denied: %w", base)

	assertEqual(t, errorx.CodeOf(wrapped), errorx.Code("AIHUB_SKILL_DELETE_DENIED"))
	assertEqual(t, errorx.HTTPStatusOf(wrapped), 403)
	assertEqual(t, errorx.MessageOf(wrapped), "没有删除权限")
	assertMapValue(t, errorx.MetadataOf(wrapped), "skill_id", "skill_001")
	if !errorx.IsKernelError(wrapped) {
		t.Fatal("wrapped kernel error should be recognized")
	}
}

func TestInspectForeignStatusCodeResponder(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("repo: %w", foreignStatusError{msg: "missing resource", status: 404})
	assertEqual(t, errorx.CodeOf(err), errorx.CodeNotFound)
	assertEqual(t, errorx.MessageOf(err), "missing resource")
	assertEqual(t, errorx.HTTPStatusOf(err), 404)
	assertEqual(t, errorx.GRPCCodeOf(err), errorx.GRPCCodeNotFound)
}

func TestInspectForeignHTTPStatusResponder(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("transport: %w", foreignHTTPStatusError{msg: "rate limited", status: 429})
	assertEqual(t, errorx.CodeOf(err), errorx.CodeTooManyRequests)
	assertEqual(t, errorx.MessageOf(err), "rate limited")
	assertEqual(t, errorx.HTTPStatusOf(err), 429)
	assertEqual(t, errorx.RetryableOf(err), true)
}

func TestInspectForeignCodedError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("pipeline: %w", foreignCodedError{})
	assertEqual(t, errorx.CodeOf(err), errorx.Code("MODEL_UPSTREAM_TIMEOUT"))
	assertEqual(t, errorx.MessageOf(err), "模型服务超时")
	assertEqual(t, errorx.HTTPStatusOf(err), 500) // custom foreign code has no status unless it exposes one
	assertEqual(t, errorx.GRPCCodeOf(err), errorx.GRPCCodeDeadlineExceeded)
	assertEqual(t, errorx.RetryableOf(err), true)
	assertEqual(t, errorx.CategoryOf(err), errorx.CategoryDependency)
	assertEqual(t, errorx.SeverityOf(err), errorx.SeverityError)
	assertMapValue(t, errorx.MetadataOf(err), "model", "qwen3")
	assertMapValue(t, errorx.PublicMetadataOf(err), "field", "model")
	assertEqual(t, errorx.RequestIDOf(err), "req_meta")
	assertEqual(t, errorx.TraceIDOf(err), "trace_meta")
}

func TestInspectForeignLogLevel(t *testing.T) {
	t.Parallel()

	err := foreignLogLevelError{}
	assertEqual(t, errorx.SeverityOf(err), errorx.SeverityWarn)
}

func TestInspectMetadataFieldCompatibility(t *testing.T) {
	t.Parallel()

	err := foreignMetadataFieldError{Metadata: map[string]any{"job_id": "job_001"}}
	assertMapValue(t, errorx.MetadataOf(err), "job_id", "job_001")
}

func TestFromNormalizesForeignErrors(t *testing.T) {
	t.Parallel()

	foreign := foreignStatusError{msg: "missing resource", status: 404}
	err := errorx.From(foreign,
		errorx.WithMetadata("resource", "skill"),
		errorx.WithRequestID("req_456"),
	)

	if err == nil {
		t.Fatal("From(non-nil) returned nil")
	}
	assertEqual(t, err.Code(), errorx.CodeNotFound)
	assertEqual(t, err.Message(), "missing resource")
	assertEqual(t, err.HTTPStatus(), 404)
	if got := err.Cause(); got != foreign {
		t.Fatalf("Cause()=%v (%T), want %v (%T)", got, got, foreign, foreign)
	}
	assertMapValue(t, err.Metadata(), "resource", "skill")
	assertEqual(t, err.RequestID(), "req_456")
}

func TestFromClonesKernelErrors(t *testing.T) {
	t.Parallel()

	base := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在", errorx.WithMetadata("skill_id", "skill_001"))
	cloned := errorx.From(base, errorx.WithMetadata("project_id", "project_001"))

	if base == cloned {
		t.Fatal("From(kernel error) should return clone, not same pointer")
	}
	assertMapValue(t, base.Metadata(), "skill_id", "skill_001")
	assertAbsent(t, base.Metadata(), "project_id")
	assertMapValue(t, cloned.Metadata(), "skill_id", "skill_001")
	assertMapValue(t, cloned.Metadata(), "project_id", "project_001")
}
