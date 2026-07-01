package retry

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestClientRetriesRetryableError(t *testing.T) {
	calls := 0
	mw := Client(WithMaxAttempts(2), WithBackoff(0))
	_, err := mw(func(context.Context, any) (any, error) {
		calls++
		if calls == 1 {
			return nil, errorx.Unavailable("UPSTREAM_DOWN", "down")
		}
		return "ok", nil
	})(context.Background(), nil)
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestClientDoesNotRetryForbidden(t *testing.T) {
	calls := 0
	mw := Client(WithMaxAttempts(3), WithBackoff(0))
	_, err := mw(func(context.Context, any) (any, error) {
		calls++
		return nil, errorx.Forbidden("AUTHZ_DENIED", "denied")
	})(context.Background(), nil)
	if err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
