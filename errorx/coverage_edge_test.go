package errorx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

type codeOnlyError struct{}

func (codeOnlyError) Error() string     { return "code only" }
func (codeOnlyError) Code() errorx.Code { return "AIHUB_CODE_ONLY" }

type stringLogLevelError struct{ level string }

func (e stringLogLevelError) Error() string    { return e.level }
func (e stringLogLevelError) LogLevel() string { return e.level }

type stringerLevel string

func (s stringerLevel) String() string { return string(s) }

type stringerLogLevelError struct{ level stringerLevel }

func (e stringerLogLevelError) Error() string           { return e.level.String() }
func (e stringerLogLevelError) LogLevel() stringerLevel { return e.level }

type stackResponderError struct{}

func (stackResponderError) Error() string { return "stack" }
func (stackResponderError) Stack() []errorx.Frame {
	return []errorx.Frame{{Function: "fn", File: "file.go", Line: 10}}
}

func TestNilErrorReceiverMethods(t *testing.T) {
	t.Parallel()

	var err *errorx.Error
	assertEqual(t, err.Error(), "<nil>")
	if err.Unwrap() != nil {
		t.Fatal("nil Error unwrap should be nil")
	}
	assertEqual(t, err.Code(), errorx.CodeInternal)
	assertEqual(t, err.ErrorCode(), "INTERNAL_ERROR")
	assertEqual(t, err.Message(), "internal server error")
	assertEqual(t, err.HTTPStatus(), 500)
	assertEqual(t, err.StatusCode(), 500)
	assertEqual(t, err.GRPCCode(), errorx.GRPCCodeInternal)
	assertEqual(t, err.Retryable(), false)
	assertEqual(t, err.Category(), errorx.CategoryInternal)
	assertEqual(t, err.Severity(), errorx.SeverityError)
	assertEqual(t, err.RequestID(), "")
	assertEqual(t, err.TraceID(), "")
	if err.Cause() != nil {
		t.Fatal("nil Error cause should be nil")
	}
	if len(err.Metadata()) != 0 || len(err.PublicMetadata()) != 0 || len(err.Stack()) != 0 {
		t.Fatal("nil Error maps should be empty and stack nil")
	}
	if err.Clone() != nil {
		t.Fatal("nil Error clone should be nil")
	}
}

func TestDefaultMessagesAndStringers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code errorx.Code
		msg  string
	}{
		{errorx.CodeOK, "success"},
		{errorx.CodeBadRequest, "bad request"},
		{errorx.CodeUnauthorized, "unauthorized"},
		{errorx.CodeForbidden, "forbidden"},
		{errorx.CodeNotFound, "not found"},
		{errorx.CodeConflict, "conflict"},
		{errorx.CodeRequestTimeout, "request timeout"},
		{errorx.CodeTooManyRequests, "too many requests"},
		{errorx.CodeClientClosedRequest, "client closed request"},
		{errorx.CodeUnavailable, "service unavailable"},
		{errorx.CodeTimeout, "timeout"},
		{errorx.CodeInternal, "internal server error"},
		{"AIHUB_CUSTOM_FAILED", "AIHUB_CUSTOM_FAILED"},
	}
	for _, tt := range cases {
		err := errorx.New(tt.code)
		assertEqual(t, err.Message(), tt.msg)
	}

	assertEqual(t, errorx.CategoryInternal.String(), "internal")
	assertEqual(t, errorx.SeverityWarn.String(), "warn")
}

func TestExtraInspectHelpers(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	err := errorx.Wrap(cause, "AIHUB_SKILL_CREATE_FAILED", errorx.WithMessage("创建失败"))
	if got := errorx.CauseOf(err); got != cause {
		t.Fatalf("CauseOf=%v, want %v", got, cause)
	}
	if got := errorx.CauseOf(fmt.Errorf("outer: %w", cause)); got != cause {
		t.Fatalf("CauseOf(foreign wrap)=%v, want %v", got, cause)
	}
	if errorx.CauseOf(nil) != nil {
		t.Fatal("CauseOf(nil) should be nil")
	}
	if !errorx.IsCode(err, "AIHUB_SKILL_CREATE_FAILED") {
		t.Fatal("IsCode should match normalized code")
	}
	if errorx.IsCode(err, "AIHUB_OTHER_FAILED") {
		t.Fatal("IsCode should not match different code")
	}

	if errorx.IsKernelError(errors.New("plain")) {
		t.Fatal("plain error should not be kernel error")
	}
}

func TestWithStatusCodeAlias(t *testing.T) {
	t.Parallel()

	err := errorx.New("AIHUB_LEGAL_RESTRICTED", errorx.WithStatusCode(451))
	assertEqual(t, errorx.HTTPStatusOf(err), 451)
}

func TestForeignCodeMethodAndLogLevelStrings(t *testing.T) {
	t.Parallel()

	assertEqual(t, errorx.CodeOf(codeOnlyError{}), errorx.Code("AIHUB_CODE_ONLY"))

	levels := map[string]errorx.Severity{
		"debug":   errorx.SeverityDebug,
		"notice":  errorx.SeverityInfo,
		"warning": errorx.SeverityWarn,
		"fatal":   errorx.SeverityError,
		"panic":   errorx.SeverityError,
	}
	for level, want := range levels {
		assertEqual(t, errorx.SeverityOf(stringLogLevelError{level: level}), want)
		assertEqual(t, errorx.SeverityOf(stringerLogLevelError{level: stringerLevel(level)}), want)
	}
	assertEqual(t, errorx.SeverityOf(stringLogLevelError{level: "unknown"}), errorx.SeverityError)
}

func TestStackOfForeignResponder(t *testing.T) {
	t.Parallel()

	frames := errorx.StackOf(stackResponderError{})
	if len(frames) != 1 {
		t.Fatalf("StackOf foreign length=%d, want 1", len(frames))
	}
	assertEqual(t, frames[0].Function, "fn")
}

func TestFormatVariants(t *testing.T) {
	t.Parallel()

	err := errorx.BadRequest("AIHUB_BAD", "坏请求")
	assertEqual(t, fmt.Sprintf("%s", err), "坏请求")
	assertEqual(t, fmt.Sprintf("%q", err), "\"坏请求\"")
}
