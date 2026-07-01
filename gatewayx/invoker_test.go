package gatewayx

import (
	"context"
	"testing"

	transport "github.com/aisphereio/kernel/transportx"
)

func TestInvokerRegistryUsesOperationAndBuildsServerContext(t *testing.T) {
	reg := NewInvokerRegistry()
	if err := reg.Register("/test.Skill/Get", UnaryInvoker(func(req DispatchRequest, match RouteMatch) (string, error) {
		return match.Params["id"], nil
	}, func(ctx context.Context, id string) (string, error) {
		tr, ok := transport.FromServerContext(ctx)
		if !ok || tr.Operation() != "/test.Skill/Get" {
			t.Fatalf("missing downstream server transport: %#v ok=%v", tr, ok)
		}
		if got := tr.RequestHeader().Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization header = %q", got)
		}
		return "skill:" + id, nil
	})); err != nil {
		t.Fatal(err)
	}

	resp, err := reg.Invoke(context.Background(), RouteMatch{
		Route:  GatewayRoute{Upstream: UpstreamRef{Operation: "/test.Skill/Get"}},
		Params: map[string]string{"id": "s1"},
	}, DispatchRequest{Headers: map[string]string{"Authorization": "Bearer token"}}, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Body != "skill:s1" {
		t.Fatalf("body=%#v", resp.Body)
	}
}
