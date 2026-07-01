package autowire

import (
	"context"
	"errors"
	"testing"
	"time"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/metadata"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/middleware/circuitbreaker"
	"github.com/aisphereio/kernel/middleware/retry"
	"github.com/aisphereio/kernel/ratelimitx"
	"github.com/aisphereio/kernel/requestx"
	transport "github.com/aisphereio/kernel/transportx"
)

type testHeader map[string][]string

func newTestHeader() testHeader { return testHeader{} }
func (h testHeader) Get(key string) string {
	if vals := h[key]; len(vals) > 0 {
		return vals[0]
	}
	return ""
}
func (h testHeader) Set(key, value string) { h[key] = []string{value} }
func (h testHeader) Add(key, value string) { h[key] = append(h[key], value) }
func (h testHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h testHeader) Values(key string) []string { return h[key] }

type testTransport struct {
	kind      transport.Kind
	endpoint  string
	operation string
	req       testHeader
	reply     testHeader
}

func (t *testTransport) Kind() transport.Kind            { return t.kind }
func (t *testTransport) Endpoint() string                { return t.endpoint }
func (t *testTransport) Operation() string               { return t.operation }
func (t *testTransport) RequestHeader() transport.Header { return t.req }
func (t *testTransport) ReplyHeader() transport.Header   { return t.reply }

func TestClientAutowireServiceGovernance(t *testing.T) {
	ctx := context.Background()
	ctx = metadata.NewServerContext(ctx, metadata.New(map[string][]string{"x-md-global-request-id": {"req-1"}}))
	tr := &testTransport{kind: transport.KindGRPC, endpoint: "discovery:///iam", operation: "/iam.v1.IAM/GetUser", req: newTestHeader(), reply: newTestHeader()}
	ctx = transport.NewClientContext(ctx, tr)

	attempts := 0
	chain := middleware.Chain(Client(
		WithClientRequestInfoResolver(func(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
			return requestx.Info{Operation: operation, Service: "iam.v1.IAM", Method: "GetUser", Exposure: accessv1.Exposure_INTERNAL, TargetService: "iam-service"}, true, nil
		}),
		WithClientMetadataPropagation(),
		WithClientTimeout(500*time.Millisecond),
		WithClientRateLimitPolicy(ratelimitx.Policy{Name: "iam-client", Enabled: true, Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 10, Burst: 2}, ratelimitx.NewMemoryProvider(), nil),
		WithRetry(retry.WithMaxAttempts(2), retry.WithBackoff(0)),
	)...)

	h := chain(func(ctx context.Context, req any) (any, error) {
		attempts++
		info, ok := requestx.FromContext(ctx)
		if !ok || info.Operation != "/iam.v1.IAM/GetUser" || info.TargetService != "iam-service" {
			t.Fatalf("request info not propagated: %#v ok=%v", info, ok)
		}
		if got := tr.req.Get("x-md-global-request-id"); got != "req-1" {
			t.Fatalf("metadata not propagated, got %q", got)
		}
		if attempts == 1 {
			return nil, errorx.Unavailable("DOWNSTREAM_UNAVAILABLE", "temporary downstream failure")
		}
		return "ok", nil
	})

	reply, err := h(ctx, "request")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "ok" || attempts != 2 {
		t.Fatalf("retry did not succeed: reply=%v attempts=%d", reply, attempts)
	}

	_, err = h(ctx, "request")
	if err != nil {
		t.Fatalf("second top-level request should consume remaining burst token, got %v", err)
	}
	_, err = h(ctx, "request")
	if err == nil || !errorx.IsTooManyRequests(err) {
		t.Fatalf("expected client-side rate limit after burst is exhausted, got %v", err)
	}
}

type openBreaker struct{}

func (openBreaker) Allow() error { return errors.New("open") }
func (openBreaker) MarkSuccess() {}
func (openBreaker) MarkFailed()  {}

func TestClientAutowireCircuitBreakerRejectsBeforeDownstream(t *testing.T) {
	ctx := transport.NewClientContext(context.Background(), &testTransport{kind: transport.KindGRPC, endpoint: "direct:///iam", operation: "/iam.v1.IAM/GetUser", req: newTestHeader(), reply: newTestHeader()})
	called := false
	chain := middleware.Chain(Client(WithCircuitBreakerFactory(func() circuitbreaker.CircuitBreaker { return openBreaker{} }))...)
	_, err := chain(func(ctx context.Context, req any) (any, error) {
		called = true
		return nil, nil
	})(ctx, nil)
	if err == nil || !errorx.IsUnavailable(err) {
		t.Fatalf("expected circuit breaker unavailable error, got %v", err)
	}
	if called {
		t.Fatal("downstream handler should not be called when breaker is open")
	}
}
