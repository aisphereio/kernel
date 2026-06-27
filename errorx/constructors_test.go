package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestShortcutConstructors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		err       error
		status    int
		grpcCode  string
		retryable bool
		category  errorx.Category
		severity  errorx.Severity
		predicate func(error) bool
	}{
		{name: "bad request", err: errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "名称不能为空"), status: 400, grpcCode: errorx.GRPCCodeInvalidArgument, retryable: false, category: errorx.CategoryValidation, severity: errorx.SeverityInfo, predicate: errorx.IsBadRequest},
		{name: "unauthorized", err: errorx.Unauthorized("IAM_TOKEN_INVALID", "未登录"), status: 401, grpcCode: errorx.GRPCCodeUnauthenticated, retryable: false, category: errorx.CategoryAuth, severity: errorx.SeverityInfo, predicate: errorx.IsUnauthorized},
		{name: "forbidden", err: errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "无权限"), status: 403, grpcCode: errorx.GRPCCodePermissionDenied, retryable: false, category: errorx.CategoryPermission, severity: errorx.SeverityInfo, predicate: errorx.IsForbidden},
		{name: "not found", err: errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "不存在"), status: 404, grpcCode: errorx.GRPCCodeNotFound, retryable: false, category: errorx.CategoryNotFound, severity: errorx.SeverityInfo, predicate: errorx.IsNotFound},
		{name: "conflict", err: errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "已存在"), status: 409, grpcCode: errorx.GRPCCodeAlreadyExists, retryable: false, category: errorx.CategoryConflict, severity: errorx.SeverityWarn, predicate: errorx.IsConflict},
		{name: "request timeout", err: errorx.RequestTimeout("HTTP_REQUEST_TIMEOUT", "请求超时"), status: 408, grpcCode: errorx.GRPCCodeDeadlineExceeded, retryable: false, category: errorx.CategoryValidation, severity: errorx.SeverityInfo, predicate: errorx.IsRequestTimeout},
		{name: "too many requests", err: errorx.TooManyRequests("MODEL_RATE_LIMITED", "限流"), status: 429, grpcCode: errorx.GRPCCodeResourceExhausted, retryable: true, category: errorx.CategoryRateLimit, severity: errorx.SeverityWarn, predicate: errorx.IsTooManyRequests},
		{name: "client closed", err: errorx.ClientClosed("HTTP_CLIENT_CLOSED_REQUEST", "客户端断开"), status: 499, grpcCode: errorx.GRPCCodeCanceled, retryable: false, category: errorx.CategoryCanceled, severity: errorx.SeverityDebug, predicate: errorx.IsClientClosedRequest},
		{name: "internal", err: errorx.Internal("AIHUB_SKILL_CREATE_FAILED", "创建失败"), status: 500, grpcCode: errorx.GRPCCodeInternal, retryable: false, category: errorx.CategoryInternal, severity: errorx.SeverityError, predicate: errorx.IsInternal},
		{name: "unavailable", err: errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "模型不可用"), status: 503, grpcCode: errorx.GRPCCodeUnavailable, retryable: true, category: errorx.CategoryDependency, severity: errorx.SeverityError, predicate: errorx.IsUnavailable},
		{name: "timeout", err: errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型超时"), status: 504, grpcCode: errorx.GRPCCodeDeadlineExceeded, retryable: true, category: errorx.CategoryDependency, severity: errorx.SeverityError, predicate: errorx.IsTimeout},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertEqual(t, errorx.HTTPStatusOf(tt.err), tt.status)
			assertEqual(t, errorx.GRPCCodeOf(tt.err), tt.grpcCode)
			assertEqual(t, errorx.RetryableOf(tt.err), tt.retryable)
			assertEqual(t, errorx.CategoryOf(tt.err), tt.category)
			assertEqual(t, errorx.SeverityOf(tt.err), tt.severity)
			if !tt.predicate(tt.err) {
				t.Fatalf("predicate returned false for %s", tt.name)
			}
		})
	}
}

func TestShortcutAliases(t *testing.T) {
	t.Parallel()

	assertEqual(t, errorx.HTTPStatusOf(errorx.InternalServer("AIHUB_INTERNAL_FAILED", "内部错误")), 500)
	assertEqual(t, errorx.HTTPStatusOf(errorx.GatewayTimeout("MODEL_TIMEOUT", "超时")), 504)
	assertEqual(t, errorx.HTTPStatusOf(errorx.ClientClosedRequest("HTTP_CLIENT_CLOSED_REQUEST", "断开")), 499)
}
