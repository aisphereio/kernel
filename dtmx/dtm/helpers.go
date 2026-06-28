package dtm

import (
	"context"
	"errors"

	"github.com/aisphereio/kernel/dtmx"
	"github.com/aisphereio/kernel/errorx"
)

func dtmxBranchURL(cfg dtmx.Config, branchPath string) string {
	return dtmx.BuildBranchURL(cfg.ServiceBaseURL, cfg.BranchPrefix, branchPath)
}

func dtmxDisabledError() error {
	return errorx.Unavailable(dtmx.CodeDisabled, "distributed transaction is disabled", errorx.WithCause(dtmx.ErrDisabled), errorx.WithRetryable(false))
}

func wrapContextError(err error, msg string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return errorx.ClientClosed(dtmx.CodeCanceled, msg, errorx.WithCause(err), errorx.WithRetryable(false))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errorx.Timeout(dtmx.CodeTimeout, msg, errorx.WithCause(err), errorx.WithRetryable(true))
	}
	return errorx.From(err, errorx.WithMessage(msg))
}
