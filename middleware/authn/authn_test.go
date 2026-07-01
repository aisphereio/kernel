package authn

import (
	"context"
	"testing"

	rootauthn "github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/errorx"
	transport "github.com/aisphereio/kernel/transportx"
)

type authenticatorFunc func(context.Context, rootauthn.Credential) (rootauthn.Principal, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, c rootauthn.Credential) (rootauthn.Principal, error) {
	return f(ctx, c)
}

func TestServerAuthenticatesAndSyncsContexts(t *testing.T) {
	req := header{"Authorization": {"Bearer token-1"}}
	tr := fakeTransport{req: req, reply: header{}}
	ctx := transport.NewServerContext(context.Background(), tr)
	mw := Server(WithAuthenticator(authenticatorFunc(func(_ context.Context, c rootauthn.Credential) (rootauthn.Principal, error) {
		if c.Token != "token-1" {
			t.Fatalf("token = %q", c.Token)
		}
		return rootauthn.Principal{SubjectID: "u1", SubjectType: rootauthn.SubjectTypeUser, TenantID: "t1", AuthMethod: rootauthn.AuthMethodJWT}, nil
	})))
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		p, ok := rootauthn.PrincipalFromContext(ctx)
		if !ok || p.SubjectID != "u1" {
			t.Fatalf("authn principal = %#v ok=%v", p, ok)
		}
		if got := contextx.SubjectIDFromContext(ctx); got != "u1" {
			t.Fatalf("contextx subject = %q", got)
		}
		if got := contextx.TenantFromContext(ctx); got != "t1" {
			t.Fatalf("tenant = %q", got)
		}
		return nil, nil
	})(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestServerMissingCredential(t *testing.T) {
	mw := Server(WithAuthenticator(authenticatorFunc(func(context.Context, rootauthn.Credential) (rootauthn.Principal, error) {
		return rootauthn.Principal{}, nil
	})))
	_, err := mw(func(context.Context, any) (any, error) { return nil, nil })(context.Background(), nil)
	if err == nil || errorx.CodeOf(err) != rootauthn.CodeMissingCredential {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

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
