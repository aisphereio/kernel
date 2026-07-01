package autowire

import (
	"context"
	"errors"
	"testing"

	"github.com/aisphereio/kernel/admissionx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/middleware/circuitbreaker"
	"github.com/aisphereio/kernel/ratelimitx"
	"github.com/aisphereio/kernel/requestx"
	transport "github.com/aisphereio/kernel/transportx"
)

func TestServerAlwaysIncludesContextInjection(t *testing.T) {
	chain := Server()
	if len(chain) != 2 {
		t.Fatalf("len(chain) = %d", len(chain))
	}
}

func TestServerPreservesOrder(t *testing.T) {
	var order []string
	mk := func(name string) middleware.Middleware {
		return func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req any) (any, error) {
				order = append(order, name+":before")
				reply, err := next(ctx, req)
				order = append(order, name+":after")
				return reply, err
			}
		}
	}
	chain := middleware.Chain(Server(WithBefore(mk("before")), WithAfter(mk("after")))...)
	_, err := chain(func(context.Context, any) (any, error) {
		order = append(order, "handler")
		return nil, nil
	})(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"before:before", "after:before", "handler", "after:after", "before:after"}
	if len(order) != len(want) {
		t.Fatalf("order = %#v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %#v want %#v", order, want)
		}
	}
}

func TestClientCircuitBreaker(t *testing.T) {
	breaker := &testBreaker{}
	chain := middleware.Chain(Client(WithCircuitBreakerFactory(func() circuitbreaker.CircuitBreaker { return breaker }))...)
	ctx := transport.NewClientContext(context.Background(), fakeClientTransport{})
	_, _ = chain(func(context.Context, any) (any, error) { return nil, errors.New("boom") })(ctx, nil)
	_, err := chain(func(context.Context, any) (any, error) { return nil, nil })(ctx, nil)
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
}

type testBreaker struct{ failed bool }

func (b *testBreaker) Allow() error {
	if b.failed {
		return errors.New("open")
	}
	return nil
}
func (b *testBreaker) MarkSuccess() { b.failed = false }
func (b *testBreaker) MarkFailed()  { b.failed = true }

type fakeClientTransport struct{}

func (fakeClientTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (fakeClientTransport) Endpoint() string                { return "demo://local" }
func (fakeClientTransport) Operation() string               { return "/demo.Service/Call" }
func (fakeClientTransport) RequestHeader() transport.Header { return header{} }
func (fakeClientTransport) ReplyHeader() transport.Header   { return header{} }

type header map[string][]string

func (h header) Get(k string) string      { return "" }
func (h header) Set(k, v string)          { h[k] = []string{v} }
func (h header) Add(k, v string)          { h[k] = append(h[k], v) }
func (h header) Values(k string) []string { return h[k] }
func (h header) Keys() []string           { return nil }

func TestClientRateLimitPolicy(t *testing.T) {
	chain := middleware.Chain(Client(WithClientRateLimitPolicy(ratelimitPolicy(), nil, nil))...)
	ctx := transport.NewClientContext(context.Background(), fakeClientTransport{})
	if _, err := chain(func(context.Context, any) (any, error) { return "ok", nil })(ctx, nil); err != nil {
		t.Fatalf("first call err=%v", err)
	}
	if _, err := chain(func(context.Context, any) (any, error) { return "ok", nil })(ctx, nil); err == nil {
		t.Fatal("expected rate limit error")
	}
}

func ratelimitPolicy() ratelimitx.Policy {
	return ratelimitx.Policy{Enabled: true, Name: "client", Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 1, Burst: 1}
}

func TestServerRequestInfoAndAdmission(t *testing.T) {
	called := false
	chain := middleware.Chain(Server(
		WithRequestInfoResolver(func(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
			return requestx.Info{Operation: "/demo.Service/Create", Action: "create"}, true, nil
		}),
		WithAdmission(admissionx.New(nil, []admissionx.ValidatingPlugin{admissionx.ValidatingPluginFunc{PluginName: "assert-info", Fn: func(ctx context.Context, a admissionx.Attributes) error {
			called = true
			if a.Request.Service != "demo.Service" || a.Request.Method != "Create" || a.Request.Action != "create" {
				t.Fatalf("unexpected request info: %#v", a.Request)
			}
			return nil
		}}})),
	)...)
	_, err := chain(func(ctx context.Context, req any) (any, error) { return "ok", nil })(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("admission plugin not called")
	}
}
