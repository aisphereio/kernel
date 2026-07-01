package requestx

import (
	"context"
	"testing"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/contextx"
)

func TestInfoNormalizeSplitsOperation(t *testing.T) {
	info := Info{Operation: "/todo.v1.TodoService/GetTodo", Exposure: accessv1.Exposure_SYSTEM}.Normalize()
	if info.Service != "todo.v1.TodoService" || info.Method != "GetTodo" {
		t.Fatalf("unexpected split: %#v", info)
	}
	if !info.IsSystem {
		t.Fatal("expected system flag")
	}
}

func TestContextRoundTripAndOperationKey(t *testing.T) {
	ctx := contextx.WithRequestID(context.Background(), "req1")
	ctx = contextx.WithTraceID(ctx, "trace1")
	info := EnrichFromContext(ctx, Info{Operation: "/svc/Method"})
	ctx = NewContext(ctx, info)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("missing info")
	}
	if got.RequestID != "req1" || got.TraceID != "trace1" {
		t.Fatalf("missing context fields: %#v", got)
	}
	if OperationKey(ctx) != "/svc/Method" {
		t.Fatalf("bad operation key: %s", OperationKey(ctx))
	}
}
