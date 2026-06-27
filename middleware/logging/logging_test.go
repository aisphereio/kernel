package logging

import (
	"context"
	"errors"
	"testing"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/transport"
)

var _ transport.Transporter = (*Transport)(nil)

type Transport struct {
	kind      transport.Kind
	endpoint  string
	operation string
}

func (tr *Transport) Kind() transport.Kind            { return tr.kind }
func (tr *Transport) Endpoint() string                { return tr.endpoint }
func (tr *Transport) Operation() string               { return tr.operation }
func (tr *Transport) RequestHeader() transport.Header { return nil }
func (tr *Transport) ReplyHeader() transport.Header   { return nil }

func TestHTTP(t *testing.T) {
	err := errors.New("reply.error")
	logger := logx.NewTestLogger(t)

	tests := []struct {
		name string
		kind func(logx.Logger) middleware.Middleware
		err  error
		ctx  context.Context
		want logx.LogLevel
	}{
		{
			name: "http-server@fail",
			kind: Server,
			err:  err,
			ctx:  transport.NewServerContext(context.Background(), &Transport{kind: transport.KindHTTP, endpoint: "endpoint", operation: "/package.service/method"}),
			want: logx.ErrorLevel,
		},
		{
			name: "http-server@succ",
			kind: Server,
			ctx:  transport.NewServerContext(context.Background(), &Transport{kind: transport.KindHTTP, endpoint: "endpoint", operation: "/package.service/method"}),
			want: logx.InfoLevel,
		},
		{
			name: "http-client@succ",
			kind: Client,
			ctx:  transport.NewClientContext(context.Background(), &Transport{kind: transport.KindHTTP, endpoint: "endpoint", operation: "/package.service/method"}),
			want: logx.InfoLevel,
		},
		{
			name: "http-client@fail",
			kind: Client,
			err:  err,
			ctx:  transport.NewClientContext(context.Background(), &Transport{kind: transport.KindHTTP, endpoint: "endpoint", operation: "/package.service/method"}),
			want: logx.ErrorLevel,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger = logx.NewTestLogger(t)
			next := func(context.Context, any) (any, error) {
				return "reply", test.err
			}
			next = test.kind(logger)(next)
			reply, gotErr := next(test.ctx, "req.args")
			if reply != "reply" {
				t.Fatalf("reply = %v, want %q", reply, "reply")
			}
			if gotErr != test.err {
				t.Fatalf("err = %v, want %v", gotErr, test.err)
			}
			entries := logger.Entries()
			if len(entries) != 1 {
				t.Fatalf("entries len = %d, want 1", len(entries))
			}
			if entries[0].Level != test.want {
				t.Fatalf("level = %v, want %v", entries[0].Level, test.want)
			}
			assertField(t, entries[0].Fields, "component", "http")
			assertField(t, entries[0].Fields, "operation", "/package.service/method")
			assertField(t, entries[0].Fields, "args", "req.args")
		})
	}
}

func assertField(t *testing.T, fields []logx.Field, key string, want any) {
	t.Helper()
	for _, field := range fields {
		if field.Key == key && field.Value == want {
			return
		}
	}
	t.Fatalf("missing field %s=%v in %#v", key, want, fields)
}

type (
	dummy                 struct{ field string }
	dummyStringer         struct{ field string }
	dummyStringerRedacter struct{ field string }
)

func (d *dummyStringer) String() string         { return "my value" }
func (d *dummyStringerRedacter) String() string { return "my value" }
func (d *dummyStringerRedacter) Redact() string { return "my value redacted" }

func TestExtractArgs(t *testing.T) {
	tests := []struct {
		name     string
		req      any
		expected string
	}{
		{name: "dummyStringer", req: &dummyStringer{field: ""}, expected: "my value"},
		{name: "dummy", req: &dummy{field: "value"}, expected: "&{field:value}"},
		{name: "dummyStringerRedacter", req: &dummyStringerRedacter{field: ""}, expected: "my value redacted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if value := extractArgs(test.req); value != test.expected {
				t.Errorf(`The stringified %s structure must be equal to %q, %v given`, test.name, test.expected, value)
			}
		})
	}
}

func TestErrorFields(t *testing.T) {
	if fields := errorFields(nil); len(fields) != 0 {
		t.Fatalf("nil error fields = %#v, want empty", fields)
	}
	fields := errorFields(errors.New("test error"))
	if len(fields) == 0 {
		t.Fatal("error fields should not be empty")
	}
}
