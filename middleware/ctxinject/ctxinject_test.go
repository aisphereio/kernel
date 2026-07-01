package ctxinject

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/middleware"
	transport "github.com/aisphereio/kernel/transportx"
)

func TestServerInjectsContextAndEchoesHeaders(t *testing.T) {
	req := header{"X-Request-ID": {"req-1"}, "X-Trace-ID": {"trace-1"}, "X-Tenant-ID": {"tenant-1"}}
	reply := header{}
	tr := fakeTransport{req: req, reply: reply}
	ctx := transport.NewServerContext(context.Background(), tr)
	mw := Server()
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		if got := contextx.RequestIDFromContext(ctx); got != "req-1" {
			t.Fatalf("request id = %q", got)
		}
		if got := contextx.TraceIDFromContext(ctx); got != "trace-1" {
			t.Fatalf("trace id = %q", got)
		}
		if got := contextx.TenantFromContext(ctx); got != "tenant-1" {
			t.Fatalf("tenant = %q", got)
		}
		return nil, nil
	})(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := reply.Get(HeaderRequestID); got != "req-1" {
		t.Fatalf("reply request id = %q", got)
	}
}

var _ middleware.Middleware = Server()

type fakeTransport struct{ req, reply header }

func (f fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f fakeTransport) Endpoint() string                { return "test://local" }
func (f fakeTransport) Operation() string               { return "/test.Service/Call" }
func (f fakeTransport) RequestHeader() transport.Header { return f.req }
func (f fakeTransport) ReplyHeader() transport.Header   { return f.reply }

type header map[string][]string

func (h header) Get(k string) string {
	if v := h[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}
func (h header) Set(k, v string)          { h[k] = []string{v} }
func (h header) Add(k, v string)          { h[k] = append(h[k], v) }
func (h header) Values(k string) []string { return h[k] }
func (h header) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
