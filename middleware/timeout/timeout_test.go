package timeout

import (
	"context"
	"testing"
	"time"

	"github.com/aisphereio/kernel/errorx"
)

func TestClientTimeoutPassesWhenFast(t *testing.T) {
	mw := Client(WithDuration(time.Second))
	_, err := mw(func(context.Context, any) (any, error) { return "ok", nil })(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientTimeoutReturnsKernelError(t *testing.T) {
	mw := Client(WithDuration(time.Nanosecond))
	_, err := mw(func(ctx context.Context, _ any) (any, error) {
		<-ctx.Done()
		return nil, nil
	})(context.Background(), nil)
	if !errorx.IsTimeout(err) {
		t.Fatalf("err=%v", err)
	}
}
