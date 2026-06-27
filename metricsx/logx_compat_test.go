package metricsx_test

import (
	"testing"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

func TestLogxLoggerCanCreateManager(t *testing.T) {
	var logger logx.Logger = logx.Noop()

	if got := metricsx.NewManager(nil, logger); got == nil {
		t.Fatal("expected no-op manager")
	}
}
