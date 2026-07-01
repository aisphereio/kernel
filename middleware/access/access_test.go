package access

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/contextx"
	transport "github.com/aisphereio/kernel/transportx"
)

func TestServerRequiresAccessAndRecordsAudit(t *testing.T) {
	store := authz.NewMemoryRelationshipStore()
	if _, err := store.WriteRelationships(context.Background(), authz.Relationship{
		Resource: authz.ObjectRef{Type: "doc", ID: "1"}, Relation: "read", Subject: authz.SubjectRef{Type: authn.SubjectTypeUser, ID: "u1"},
	}); err != nil {
		t.Fatal(err)
	}
	audit := auditx.NewMemoryStore()
	guard := accessx.New(nil, authz.NewMemoryAuthorizer(store), audit)
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req-1")
	ctx = contextx.WithTraceID(ctx, "trace-1")
	ctx = authn.ContextWithPrincipal(ctx, authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser})
	ctx = transport.NewServerContext(ctx, fakeTransport{req: header{"User-Agent": {"test-agent"}}, reply: header{}, operation: "/doc.Service/Get"})
	mw := Server(guard, WithResolver(func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
		if operation != "/doc.Service/Get" {
			t.Fatalf("operation = %q", operation)
		}
		return accessx.Check{Permission: "read", Resource: authz.ObjectRef{Type: "doc", ID: "1"}, AuditAction: "doc.access"}, true, nil
	}))
	_, err := mw(func(context.Context, any) (any, error) { return "ok", nil })(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	records, err := audit.Query(context.Background(), auditx.QueryFilter{ActorID: "u1", Action: "doc.access"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].RequestID != "req-1" || records[0].TraceID != "trace-1" || records[0].UserAgent != "test-agent" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

type fakeTransport struct {
	req, reply header
	operation  string
}

func (f fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f fakeTransport) Endpoint() string                { return "test://local" }
func (f fakeTransport) Operation() string               { return f.operation }
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
