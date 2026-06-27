package contextx_test

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/logx"
)

func TestLogxLoggerCanBeAttachedToContext(t *testing.T) {
	var logger logx.Logger = logx.Noop()

	ctx := contextx.WithLogger(context.Background(), logger)

	if got := contextx.LoggerFromContext(ctx); got == nil {
		t.Fatal("expected logger from context")
	}
}
