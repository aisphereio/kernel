package requestinfo

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/requestx"
)

func TestServerResolverAttachesInfo(t *testing.T) {
	mw := Server(WithResolver(func(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
		return requestx.Info{Operation: "/svc/Method", Action: "read"}, true, nil
	}))
	h := mw(func(ctx context.Context, req any) (any, error) {
		info, ok := requestx.FromContext(ctx)
		if !ok {
			t.Fatal("missing request info")
		}
		if info.Service != "svc" || info.Method != "Method" || info.Action != "read" {
			t.Fatalf("bad info: %#v", info)
		}
		return "ok", nil
	})
	if _, err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	_ = middleware.Handler(nil)
}
