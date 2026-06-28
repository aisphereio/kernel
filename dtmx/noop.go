package dtmx

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/errorx"
)

type disabledManager struct{}

func Disabled() Manager { return disabledManager{} }

func (disabledManager) Enabled() bool           { return false }
func (disabledManager) DriverName() string      { return "disabled" }
func (disabledManager) Close() error            { return nil }
func (disabledManager) BranchURL(string) string { return "" }

func (disabledManager) NewGID(context.Context) (string, error) {
	return "", normalizeError(ErrDisabled, "distributed transaction is disabled")
}

func (disabledManager) SubmitSaga(context.Context, Saga) (TransactionResult, error) {
	return TransactionResult{Pattern: "saga", Protocol: ProtocolHTTP, StartedAt: time.Now()}, normalizeError(ErrDisabled, "distributed transaction is disabled", errorx.WithRetryable(false))
}
