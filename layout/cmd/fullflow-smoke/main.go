package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	stdhttp "net/http"
	"strings"
	"sync/atomic"
	"time"

	v1 "github.com/aisphereio/kernel-layout/api/todo/v1"
	"github.com/aisphereio/kernel-layout/internal/biz"
	"github.com/aisphereio/kernel-layout/internal/data"
	"github.com/aisphereio/kernel-layout/internal/service"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/admissionx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/middleware"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/middleware/autowire"
	"github.com/aisphereio/kernel/middleware/circuitbreaker"
	"github.com/aisphereio/kernel/middleware/ctxinject"
	"github.com/aisphereio/kernel/middleware/ratelimit"
	transport "github.com/aisphereio/kernel/transportx"
	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
	khttp "github.com/aisphereio/kernel/transportx/http"
	"google.golang.org/grpc"
	grpcmd "google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type step struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Note string `json:"note,omitempty"`
}

type smokeResult struct {
	HTTPAddr     string `json:"http_addr"`
	GRPCAddr     string `json:"grpc_addr"`
	CreatedID    int64  `json:"created_id"`
	AuditRecords int    `json:"audit_records"`
	Steps        []step `json:"steps"`
}

type countLimiter struct{ remaining atomic.Int64 }

func newCountLimiter(n int64) *countLimiter {
	l := &countLimiter{}
	l.remaining.Store(n)
	return l
}

func (l *countLimiter) Allow() (ratelimit.DoneFunc, error) {
	for {
		cur := l.remaining.Load()
		if cur <= 0 {
			return nil, errors.New("demo limiter exhausted")
		}
		if l.remaining.CompareAndSwap(cur, cur-1) {
			return func(ratelimit.DoneInfo) {}, nil
		}
	}
}

type openAfterFailuresBreaker struct{ failures atomic.Int64 }

func (b *openAfterFailuresBreaker) Allow() error {
	if b.failures.Load() >= 1 {
		return errors.New("demo circuit open")
	}
	return nil
}
func (b *openAfterFailuresBreaker) MarkSuccess() { b.failures.Store(0) }
func (b *openAfterFailuresBreaker) MarkFailed()  { b.failures.Add(1) }

func demoAdmissionChain() admissionx.Chain {
	return admissionx.New(
		[]admissionx.MutatingPlugin{
			admissionx.MutatingPluginFunc{PluginName: "todo.default-title", Fn: func(ctx context.Context, a admissionx.Attributes) (any, error) {
				_ = ctx
				req, ok := a.Object.(*v1.CreateTodoRequest)
				if !ok || req.GetTodo() == nil || strings.TrimSpace(req.GetTodo().GetTitle()) != "" {
					return a.Object, nil
				}
				clone := &v1.CreateTodoRequest{Todo: req.GetTodo()}
				clone.Todo.Title = "admission-default-title"
				return clone, nil
			}},
		},
		[]admissionx.ValidatingPlugin{
			admissionx.ValidatingPluginFunc{PluginName: "todo.deny-title", Fn: func(ctx context.Context, a admissionx.Attributes) error {
				_ = ctx
				req, ok := a.Object.(*v1.CreateTodoRequest)
				if !ok || req.GetTodo() == nil {
					return nil
				}
				if strings.TrimSpace(req.GetTodo().GetTitle()) == "admission-deny" {
					return errors.New("title is denied by admission policy")
				}
				return nil
			}},
		},
	)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _, err := logx.New(logx.Config{ServiceName: "kernel-fullflow-smoke", Env: "demo", Version: "dev", Level: "warn", Format: "console", Output: "stdout"})
	must("init logger", err)
	defer func() { _ = logger.Sync() }()

	metrics := metricsx.Noop()
	auditStore := auditx.NewMemoryStore()
	guard := newDemoGuard(context.Background(), auditStore)

	repoData := data.NewData(&data.Resources{Audit: auditStore, Authz: guard.Authz, Access: guard})
	todoSvc := service.NewTodoService(biz.NewTodoUsecase(data.NewTodoRepo(repoData)))

	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	must("listen http", err)
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	must("listen grpc", err)

	commonMiddleware := autowire.Server(
		autowire.WithContextInjection(
			ctxinject.WithTenantHeaders("X-Demo-Tenant", "x-demo-tenant", "X-Tenant-ID", "x-tenant-id"),
			ctxinject.WithReplyHeaders("X-Demo-Request-ID", "X-Demo-Trace-ID", "X-Demo-Tenant"),
		),
		autowire.WithAuthn(demoAuthenticator{}),
		autowire.WithCredentialExtractor(authnmw.HeaderExtractor("X-Demo-Subject", authn.CredentialAPIKey)),
		autowire.WithRequestInfoResolver(v1.TodoServiceRequestInfoResolver),
		autowire.WithAccess(guard, v1.TodoServiceAccessResolver),
		autowire.WithAdmission(demoAdmissionChain()),
	)

	httpSrv := khttp.NewServer(
		khttp.Listener(httpLis),
		khttp.Timeout(2*time.Second),
		khttp.Logger(logger.Named("http")),
		khttp.Metrics(metrics),
		khttp.Middleware(commonMiddleware...),
	)
	httpSrv.Use(v1.OperationTodoServiceListTodos, ratelimit.Server(ratelimit.WithLimiter(newCountLimiter(2))))
	v1.RegisterTodoServiceHTTPServer(httpSrv, todoSvc)
	httpSrv.HandleFunc("/debug/context", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, 200, map[string]string{
			"request_id": contextx.RequestIDFromContext(r.Context()),
			"subject_id": contextx.SubjectIDFromContext(r.Context()),
			"tenant_id":  contextx.TenantFromContext(r.Context()),
		})
	})

	grpcSrv := kgrpc.NewServer(
		kgrpc.Listener(grpcLis),
		kgrpc.Timeout(2*time.Second),
		kgrpc.Logger(logger.Named("grpc")),
		kgrpc.Metrics(metrics),
		kgrpc.Middleware(commonMiddleware...),
	)
	grpcSrv.Use(v1.OperationTodoServiceListTodos, ratelimit.Server(ratelimit.WithLimiter(newCountLimiter(1))))
	v1.RegisterTodoServiceServer(grpcSrv, todoSvc)

	errCh := make(chan error, 2)
	go func() { errCh <- httpSrv.Start(ctx) }()
	go func() { errCh <- grpcSrv.Start(ctx) }()
	time.Sleep(250 * time.Millisecond)

	result := smokeResult{HTTPAddr: httpLis.Addr().String(), GRPCAddr: grpcLis.Addr().String()}
	add := func(name string, ok bool, note string) {
		result.Steps = append(result.Steps, step{Name: name, OK: ok, Note: note})
	}

	httpClient := &stdhttp.Client{Timeout: 3 * time.Second}
	headers := map[string]string{
		"Content-Type":    "application/json",
		"Accept":          "application/json",
		"X-Request-ID":    "req-http-001",
		"X-Trace-ID":      "trace-http-001",
		"X-Demo-Subject":  "demo-user",
		"X-Demo-Tenant":   "tenant-a",
		"X-Demo-AuthMode": "dev_token",
	}

	createResp, code, h, err := doJSON(httpClient, "POST", "http://"+result.HTTPAddr+"/v1/todos/create", headers, map[string]any{"title": "kernel fullflow", "content": "http create"})
	mustStep(add, "http create todo", err)
	add("http ctx reply header", h.Get("X-Demo-Request-ID") == "req-http-001", h.Get("X-Demo-Request-ID"))
	if code != 200 {
		fail("http create status", fmt.Errorf("status=%d body=%s", code, createResp))
	}
	var created v1.Todo
	must("decode http create", json.Unmarshal(createResp, &created))
	result.CreatedID = created.GetId()
	add("http create response", created.GetId() == 1 && created.GetTitle() == "kernel fullflow", fmt.Sprintf("id=%d title=%q", created.GetId(), created.GetTitle()))

	defaultedResp, code, _, err := doJSON(httpClient, "POST", "http://"+result.HTTPAddr+"/v1/todos/create", headers, map[string]any{"content": "admission default"})
	mustStep(add, "admission mutating default request", err)
	var defaulted v1.Todo
	if code == 200 {
		must("decode admission default", json.Unmarshal(defaultedResp, &defaulted))
	}
	add("admission default title", code == 200 && defaulted.GetTitle() == "admission-default-title", fmt.Sprintf("status=%d title=%q body=%s", code, defaulted.GetTitle(), compact(defaultedResp)))

	deniedResp, code, _, err := doJSON(httpClient, "POST", "http://"+result.HTTPAddr+"/v1/todos/create", headers, map[string]any{"title": "admission-deny", "content": "should reject"})
	mustStep(add, "admission validating request", err)
	add("admission reject title", code >= 400 && strings.Contains(string(deniedResp), "ADMISSION_VALIDATION_DENIED"), fmt.Sprintf("status=%d body=%s", code, compact(deniedResp)))

	getBody, code, _, err := doJSON(httpClient, "GET", fmt.Sprintf("http://%s/v1/todos/%d", result.HTTPAddr, created.GetId()), headers, nil)
	mustStep(add, "http get todo", err)
	add("http get status", code == 200, fmt.Sprintf("status=%d body=%s", code, compact(getBody)))

	_, code, _, err = doJSON(httpClient, "GET", "http://"+result.HTTPAddr+"/v1/todos/list", headers, nil)
	mustStep(add, "http list allowed #1", err)
	add("http list allowed #1 status", code == 200, fmt.Sprintf("status=%d", code))
	_, code, _, err = doJSON(httpClient, "GET", "http://"+result.HTTPAddr+"/v1/todos/list", headers, nil)
	mustStep(add, "http list allowed #2", err)
	add("http list allowed #2 status", code == 200, fmt.Sprintf("status=%d", code))
	limitedBody, code, _, err := doJSON(httpClient, "GET", "http://"+result.HTTPAddr+"/v1/todos/list", headers, nil)
	mustStep(add, "http list rate limited request", err)
	add("http rate limit status/code", code == 429 && bytes.Contains(limitedBody, []byte("RATE_LIMIT_EXCEEDED")), fmt.Sprintf("status=%d body=%s", code, compact(limitedBody)))

	deniedHeaders := cloneHeaders(headers)
	deniedHeaders["X-Demo-Subject"] = "bad-user"
	deniedBody, code, _, err := doJSON(httpClient, "GET", fmt.Sprintf("http://%s/v1/todos/%d", result.HTTPAddr, created.GetId()), deniedHeaders, nil)
	mustStep(add, "http authz denied request", err)
	add("http authz denied status/code", code == 403 && bytes.Contains(deniedBody, []byte("AUTHZ_PERMISSION_DENIED")), fmt.Sprintf("status=%d body=%s", code, compact(deniedBody)))

	// gRPC calls use Kernel's grpcx client so client-side middleware, metadata propagation,
	// selector balancer initialization, and errorx-from-gRPC adaptation are exercised.
	conn, err := kgrpc.NewClient(ctx,
		kgrpc.WithEndpoint("direct:///"+result.GRPCAddr),
		kgrpc.WithTimeout(2*time.Second),
		kgrpc.WithHealthCheck(false),
	)
	must("grpcx client", err)
	defer conn.Close()
	grpcClient := v1.NewTodoServiceClient(conn)

	grpcCtx := grpcmd.AppendToOutgoingContext(ctx,
		"x-request-id", "req-grpc-001",
		"x-trace-id", "trace-grpc-001",
		"x-demo-subject", "demo-user",
		"x-demo-tenant", "tenant-a",
		"x-demo-authmode", "dev_token",
	)
	var replyHeader grpcmd.MD
	got, err := grpcClient.GetTodo(grpcCtx, &v1.GetTodoRequest{Id: created.GetId()}, grpc.Header(&replyHeader))
	mustStep(add, "grpc get todo", err)
	add("grpc ctx reply header", firstMD(replyHeader, "x-demo-request-id") == "req-grpc-001", firstMD(replyHeader, "x-demo-request-id"))
	add("grpc get response", got.GetId() == created.GetId(), fmt.Sprintf("id=%d title=%q", got.GetId(), got.GetTitle()))

	_, err = grpcClient.UpdateTodo(grpcCtx, &v1.UpdateTodoRequest{Todo: &v1.Todo{Id: created.GetId(), Title: "kernel fullflow updated", Content: "grpc update"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title", "content"}}})
	mustStep(add, "grpc update todo", err)

	_, err = grpcClient.ListTodos(grpcCtx, &v1.ListTodosRequest{PageSize: 10})
	mustStep(add, "grpc list allowed", err)
	_, err = grpcClient.ListTodos(grpcCtx, &v1.ListTodosRequest{PageSize: 10})
	add("grpc rate limit errorx", err != nil && errorx.HTTPStatusOf(err) == 429 && errorx.CodeOf(err).String() == "RATE_LIMIT_EXCEEDED", fmt.Sprintf("err=%v code=%s status=%d", err, errorx.CodeOf(err), errorx.HTTPStatusOf(err)))

	_, err = grpcClient.GetTodo(grpcCtx, &v1.GetTodoRequest{Id: 404404})
	add("grpc not found errorx mapping", err != nil && errorx.CodeOf(err).String() == v1.ErrorReason_TODO_NOT_FOUND.String() && errorx.HTTPStatusOf(err) == 404, fmt.Sprintf("err=%v code=%s status=%d", err, errorx.CodeOf(err), errorx.HTTPStatusOf(err)))

	breaker := &openAfterFailuresBreaker{}
	cb := middleware.Chain(autowire.Client(autowire.WithCircuitBreakerFactory(func() circuitbreaker.CircuitBreaker { return breaker }))...)
	cbHandler := cb(func(context.Context, any) (any, error) {
		return nil, errorx.Unavailable("DEMO_BACKEND_DOWN", "demo backend unavailable")
	})
	cbCtx := transport.NewClientContext(ctx, demoClientTransport{operation: "/demo.Service/Call"})
	_, _ = cbHandler(cbCtx, nil)
	_, err = cbHandler(cbCtx, nil)
	add("circuit breaker middleware open", err != nil && errorx.HTTPStatusOf(err) == 503 && errorx.CodeOf(err).String() == "CIRCUIT_BREAKER_OPEN", fmt.Sprintf("err=%v code=%s status=%d", err, errorx.CodeOf(err), errorx.HTTPStatusOf(err)))

	records, err := auditStore.Query(ctx, auditx.QueryFilter{ActorID: "demo-user", Action: "todo.access"})
	must("query audit", err)
	result.AuditRecords = len(records)
	add("audit records for successful principal", len(records) >= 6, fmt.Sprintf("records=%d", len(records)))

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	_ = httpSrv.Stop(stopCtx)
	_ = grpcSrv.Stop(stopCtx)
	cancel()
	select {
	case <-errCh:
	case <-time.After(200 * time.Millisecond):
	}

	for _, s := range result.Steps {
		if !s.OK {
			b, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(b))
			panic("fullflow smoke failed at: " + s.Name)
		}
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}

func newDemoGuard(ctx context.Context, audit auditx.Recorder) accessx.Guard {
	store := authz.NewMemoryRelationshipStore()
	rels := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "todo", ID: "collection"}, Relation: "create", Subject: authz.SubjectRef{Type: authz.SubjectTypeUser, ID: "demo-user"}},
		{Resource: authz.ObjectRef{Type: "todo", ID: "collection"}, Relation: "read", Subject: authz.SubjectRef{Type: authz.SubjectTypeUser, ID: "demo-user"}},
		{Resource: authz.ObjectRef{Type: "todo", ID: "1"}, Relation: "read", Subject: authz.SubjectRef{Type: authz.SubjectTypeUser, ID: "demo-user"}},
		{Resource: authz.ObjectRef{Type: "todo", ID: "1"}, Relation: "update", Subject: authz.SubjectRef{Type: authz.SubjectTypeUser, ID: "demo-user"}},
		{Resource: authz.ObjectRef{Type: "todo", ID: "1"}, Relation: "delete", Subject: authz.SubjectRef{Type: authz.SubjectTypeUser, ID: "demo-user"}},
		{Resource: authz.ObjectRef{Type: "todo", ID: "404404"}, Relation: "read", Subject: authz.SubjectRef{Type: authz.SubjectTypeUser, ID: "demo-user"}},
	}
	if _, err := store.WriteRelationships(ctx, rels...); err != nil {
		panic(err)
	}
	return accessx.New(nil, authz.NewMemoryAuthorizer(store), audit)
}

type demoAuthenticator struct{}

func (demoAuthenticator) Authenticate(_ context.Context, credential authn.Credential) (authn.Principal, error) {
	subject := strings.TrimSpace(credential.Token)
	if subject == "" {
		return authn.Principal{}, authn.ErrMissingCredential("")
	}
	return authn.Principal{
		SubjectID:   subject,
		SubjectType: authn.SubjectTypeUser,
		AuthMethod:  authn.AuthMethodDevToken,
		Roles:       []string{"demo"},
		Scopes:      []string{"todo:*"},
	}, nil
}

func doJSON(c *stdhttp.Client, method, url string, headers map[string]string, body any) ([]byte, int, stdhttp.Header, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := stdhttp.NewRequest(method, url, rd)
	if err != nil {
		return nil, 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, resp.Header, err
}

func writeJSON(w stdhttp.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func must(label string, err error) {
	if err != nil {
		panic(label + ": " + err.Error())
	}
}
func mustStep(add func(string, bool, string), label string, err error) {
	add(label, err == nil, fmt.Sprint(err))
}
func fail(label string, err error) { panic(label + ": " + err.Error()) }
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
func firstHeader(h transport.Header, names ...string) string {
	if h == nil {
		return ""
	}
	for _, n := range names {
		if v := h.Get(n); v != "" {
			return v
		}
	}
	return ""
}
func cloneHeaders(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func compact(b []byte) string { return strings.Join(strings.Fields(string(b)), " ") }
func firstMD(md grpcmd.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

type demoClientTransport struct{ operation string }

func (d demoClientTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (d demoClientTransport) Endpoint() string                { return "demo://local" }
func (d demoClientTransport) Operation() string               { return d.operation }
func (d demoClientTransport) RequestHeader() transport.Header { return demoHeader{} }
func (d demoClientTransport) ReplyHeader() transport.Header   { return demoHeader{} }

type demoHeader map[string][]string

func (h demoHeader) Get(k string) string {
	if v := h[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}
func (h demoHeader) Set(k, v string) { h[k] = []string{v} }
func (h demoHeader) Add(k, v string) { h[k] = append(h[k], v) }
func (h demoHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h demoHeader) Values(k string) []string { return h[k] }
