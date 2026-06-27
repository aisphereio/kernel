package errorx_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestNewCreatesCanonicalError(t *testing.T) {
	t.Parallel()

	err := errorx.New("AIHUB_SKILL_CREATE_FAILED",
		errorx.WithMessage("创建技能失败"),
		errorx.WithHTTPStatus(errorx.HTTPStatusInternalServerError),
		errorx.WithGRPCCode(errorx.GRPCCodeInternal),
		errorx.WithRetryable(true),
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithPublicMetadata("field", "name"),
		errorx.WithRequestID("req_123"),
		errorx.WithTraceID("trace_abc"),
		errorx.WithCategory(errorx.CategoryDependency),
		errorx.WithSeverity(errorx.SeverityError),
	)

	assertEqual(t, err.Code(), errorx.Code("AIHUB_SKILL_CREATE_FAILED"))
	assertEqual(t, err.Error(), "创建技能失败")
	assertEqual(t, err.Message(), "创建技能失败")
	assertEqual(t, err.HTTPStatus(), errorx.HTTPStatusInternalServerError)
	assertEqual(t, err.StatusCode(), errorx.HTTPStatusInternalServerError)
	assertEqual(t, err.GRPCCode(), errorx.GRPCCodeInternal)
	assertEqual(t, err.Retryable(), true)
	assertEqual(t, err.RequestID(), "req_123")
	assertEqual(t, err.TraceID(), "trace_abc")
	assertEqual(t, err.Category(), errorx.CategoryDependency)
	assertEqual(t, err.Severity(), errorx.SeverityError)
	assertMapValue(t, err.Metadata(), "skill_id", "skill_001")
	assertMapValue(t, err.PublicMetadata(), "field", "name")
}

func TestErrorDefensiveCopyAndClone(t *testing.T) {
	t.Parallel()

	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithPublicMetadata("field", "id"),
	)

	md := err.Metadata()
	md["skill_id"] = "tampered"
	assertMapValue(t, err.Metadata(), "skill_id", "skill_001")

	public := err.PublicMetadata()
	public["field"] = "tampered"
	assertMapValue(t, err.PublicMetadata(), "field", "id")

	clone := err.Clone()
	cloneMD := clone.Metadata()
	cloneMD["skill_id"] = "clone_tampered"
	assertMapValue(t, err.Metadata(), "skill_id", "skill_001")
	assertMapValue(t, clone.Metadata(), "skill_id", "skill_001")
}

func TestWrapPreservesCauseAndErrorChain(t *testing.T) {
	t.Parallel()

	cause := errors.New("pq: connection refused")
	err := errorx.Wrap(cause, "AIHUB_SKILL_CREATE_FAILED",
		errorx.WithMessage("创建技能失败"),
		errorx.WithRetryable(true),
	)

	if err == nil {
		t.Fatal("Wrap returned nil for non-nil cause")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(wrapped, cause)=false")
	}
	var kernelErr *errorx.Error
	if !errors.As(err, &kernelErr) {
		t.Fatalf("errors.As(wrapped, *errorx.Error)=false")
	}
	assertEqual(t, kernelErr.Code(), errorx.Code("AIHUB_SKILL_CREATE_FAILED"))
	assertEqual(t, kernelErr.Cause(), cause)
	assertEqual(t, errorx.RetryableOf(err), true)
}

func TestWrapNilReturnsNil(t *testing.T) {
	t.Parallel()

	if err := errorx.Wrap(nil, "AIHUB_SKILL_CREATE_FAILED"); err != nil {
		t.Fatalf("Wrap(nil)=%v, want nil", err)
	}
}

func TestErrorsIsMatchesByCode(t *testing.T) {
	t.Parallel()

	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	target := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "另一条消息")
	if !errors.Is(err, target) {
		t.Fatal("errors.Is should match errorx errors by code")
	}
	other := errorx.NotFound("AIHUB_PROJECT_NOT_FOUND", "项目不存在")
	if errors.Is(err, other) {
		t.Fatal("errors.Is should not match different codes")
	}
}

func TestFormatPlusVIncludesDebugFields(t *testing.T) {
	t.Parallel()

	err := errorx.Internal("AIHUB_INTERNAL_FAILED", "内部错误",
		errorx.WithRequestID("req_123"),
		errorx.WithTraceID("trace_abc"),
	)
	formatted := fmt.Sprintf("%+v", err)
	for _, want := range []string{"error_code=AIHUB_INTERNAL_FAILED", "http_status=500", "request_id=req_123", "trace_id=trace_abc"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error missing %q: %s", want, formatted)
		}
	}
}
