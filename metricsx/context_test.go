package metricsx_test

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/metricsx"
)

func TestContextRoundTrip(t *testing.T) {
	manager := metricsx.Noop()
	ctx := metricsx.Inject(context.Background(), manager)
	if got := metricsx.FromContext(ctx); got != manager {
		t.Fatalf("FromContext() = %#v, want %#v", got, manager)
	}
}

func TestContextFallbacksAreNoop(t *testing.T) {
	if got := metricsx.FromContext(context.Background()); got == nil {
		t.Fatal("FromContext should return no-op manager, got nil")
	}
	if got := metricsx.FromContextOr(context.Background(), nil); got == nil {
		t.Fatal("FromContextOr should return no-op manager, got nil")
	}
	if got := metricsx.Ensure(nil); got == nil {
		t.Fatal("Ensure should return no-op manager, got nil")
	}
}
