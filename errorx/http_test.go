package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestDefaultHTTPAndGRPCMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code     errorx.Code
		status   int
		grpcCode string
	}{
		{errorx.CodeOK, 200, errorx.GRPCCodeOK},
		{errorx.CodeBadRequest, 400, errorx.GRPCCodeInvalidArgument},
		{errorx.CodeUnauthorized, 401, errorx.GRPCCodeUnauthenticated},
		{errorx.CodeForbidden, 403, errorx.GRPCCodePermissionDenied},
		{errorx.CodeNotFound, 404, errorx.GRPCCodeNotFound},
		{errorx.CodeConflict, 409, errorx.GRPCCodeAlreadyExists},
		{errorx.CodeRequestTimeout, 408, errorx.GRPCCodeDeadlineExceeded},
		{errorx.CodeTooManyRequests, 429, errorx.GRPCCodeResourceExhausted},
		{errorx.CodeClientClosedRequest, 499, errorx.GRPCCodeCanceled},
		{errorx.CodeInternal, 500, errorx.GRPCCodeInternal},
		{errorx.CodeUnavailable, 503, errorx.GRPCCodeUnavailable},
		{errorx.CodeTimeout, 504, errorx.GRPCCodeDeadlineExceeded},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.code.String(), func(t *testing.T) {
			t.Parallel()
			err := errorx.New(tt.code)
			assertEqual(t, errorx.HTTPStatusOf(err), tt.status)
			assertEqual(t, errorx.GRPCCodeOf(err), tt.grpcCode)
		})
	}
}

func TestHTTPStatusOptionOverridesDefault(t *testing.T) {
	t.Parallel()

	err := errorx.New("AIHUB_CUSTOM_DENIED", errorx.WithHTTPStatus(451), errorx.WithGRPCCode("Custom"))
	assertEqual(t, errorx.HTTPStatusOf(err), 451)
	assertEqual(t, errorx.GRPCCodeOf(err), "Custom")
}

func TestInvalidHTTPStatusOptionIgnored(t *testing.T) {
	t.Parallel()

	err := errorx.New(errorx.CodeBadRequest, errorx.WithHTTPStatus(0))
	assertEqual(t, errorx.HTTPStatusOf(err), 400)
}
