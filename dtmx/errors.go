package dtmx

import (
	"errors"

	"github.com/aisphereio/kernel/errorx"
)

var (
	ErrDisabled            = errors.New("dtmx: distributed transaction is disabled")
	ErrInvalidConfig       = errors.New("dtmx: invalid config")
	ErrUnknownDriver       = errors.New("dtmx: unknown driver")
	ErrNoSteps             = errors.New("dtmx: saga has no steps")
	ErrInvalidBranch       = errors.New("dtmx: invalid saga branch")
	ErrGIDRequired         = errors.New("dtmx: gid is required")
	ErrUnsupportedProtocol = errors.New("dtmx: unsupported transaction protocol")
	ErrSubmitFailed        = errors.New("dtmx: submit transaction failed")
	ErrNewGIDFailed        = errors.New("dtmx: create gid failed")
)

const (
	CodeDisabled            = errorx.Code("DTMX_DISABLED")
	CodeInvalidConfig       = errorx.Code("DTMX_INVALID_CONFIG")
	CodeUnknownDriver       = errorx.Code("DTMX_UNKNOWN_DRIVER")
	CodeInvalidSaga         = errorx.Code("DTMX_INVALID_SAGA")
	CodeUnsupportedProtocol = errorx.Code("DTMX_UNSUPPORTED_PROTOCOL")
	CodeSubmitFailed        = errorx.Code("DTMX_SUBMIT_FAILED")
	CodeNewGIDFailed        = errorx.Code("DTMX_NEW_GID_FAILED")
	CodeTimeout             = errorx.Code("DTMX_TIMEOUT")
	CodeCanceled            = errorx.Code("DTMX_CANCELED")
	CodeBranchUnauthorized  = errorx.Code("DTMX_BRANCH_UNAUTHORIZED")
)

func normalizeError(err error, msg string, opts ...errorx.Option) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrDisabled):
		return errorx.Unavailable(CodeDisabled, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	case errors.Is(err, ErrInvalidConfig):
		return errorx.BadRequest(CodeInvalidConfig, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	case errors.Is(err, ErrUnknownDriver):
		return errorx.BadRequest(CodeUnknownDriver, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	case errors.Is(err, ErrNoSteps), errors.Is(err, ErrInvalidBranch), errors.Is(err, ErrGIDRequired):
		return errorx.BadRequest(CodeInvalidSaga, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	case errors.Is(err, ErrUnsupportedProtocol):
		return errorx.BadRequest(CodeUnsupportedProtocol, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	case errors.Is(err, contextCanceledSentinel):
		return errorx.ClientClosed(CodeCanceled, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	case errors.Is(err, contextTimeoutSentinel):
		return errorx.Timeout(CodeTimeout, msg, append([]errorx.Option{errorx.WithCause(err)}, opts...)...)
	default:
		return errorx.Unavailable(CodeSubmitFailed, msg, append([]errorx.Option{errorx.WithCause(err), errorx.WithRetryable(true)}, opts...)...)
	}
}

// Internal sentinels let normalizeError map context errors without importing
// context from files that only define package-level contracts. They are assigned
// in context_errors.go.
var (
	contextCanceledSentinel error
	contextTimeoutSentinel  error
)
