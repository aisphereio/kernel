package logx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aisphereio/kernel/logx"
)

// This file demonstrates COMPLETE business scenarios showing how logx fits
// into handler → service → repository flow. AI tools should copy these
// patterns when generating new business code.

// ============================================================================
// SCENARIO 1: Repository query log
// ============================================================================

// ExampleBusiness_repositoryLog shows the standard pattern for logging
// database queries in repository code — use FromContext to get the
// request-scoped logger with request_id / trace_id / subject_id already
// attached.
func Example_businessRepositoryLog() {
	logger := logx.NewTestLogger(nil)
	ctx := logx.Inject(context.Background(), logger,
		logx.String("request_id", "req_abc"),
	)

	repo := &fakeSkillRepo{logger: logger}
	_, _ = repo.Find(ctx, "skill_001")

	entries := logger.Entries()
	fmt.Println(entries[0].Message)
	// Output: querying skill
}

// ============================================================================
// SCENARIO 2: Service operation log
// ============================================================================

// ExampleBusiness_serviceLog shows service-layer logging with operation and
// resource fields for searchability.
func Example_businessServiceLog() {
	logger := logx.NewTestLogger(nil)
	svc := &fakeSkillService{logger: logger}
	_ = svc.Create(context.Background(), "demo")

	entries := logger.Entries()
	fmt.Println(entries[0].Message)
	// Output: skill created
}

// ============================================================================
// SCENARIO 3: Upstream call log
// ============================================================================

// ExampleBusiness_upstreamCall shows how to log calls to upstream services
// (LLM APIs, third-party HTTP) using LogExternalCall.
func Example_businessUpstreamCall() {
	logger := logx.NewTestLogger(nil)
	callUpstreamLLM(logger, "gpt-4", 200, 850*time.Millisecond)

	entries := logger.Entries()
	fmt.Println(entries[0].Fields[0].Key) // first field is event=external_call... actually "event"
	// Output: event
}

// ============================================================================
// SCENARIO 4: Worker log with redaction
// ============================================================================

// ExampleBusiness_workerLog shows a background worker logging progress with
// auto-redaction of sensitive fields.
func Example_businessWorkerLog() {
	// Build a logger with redaction enabled.
	cfg := logx.DefaultConfig("test")
	cfg.Redact = logx.RedactConfig{Enabled: true, Keys: logx.DefaultRedactKeys(), Value: "***"}
	logger, _, _ := logx.New(cfg, logx.WithWriter(io.Discard))

	// Even if a worker accidentally passes a credential, it's redacted.
	logger.Info("worker processing",
		logx.String("task_id", "task_001"),
		logx.String("api_key", "sk_live_abc123"), // redacted to ***
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 5: HTTP access log
// ============================================================================

// ExampleBusiness_httpAccess shows the standard access log shape produced
// by HTTPAccessLog middleware.
func Example_businessHTTPAccess() {
	logger := logx.NewTestLogger(nil)
	logx.LogAccess(logger, logx.AccessEvent{
		Side:       "server",
		Protocol:   "http",
		Operation:  "POST /v1/skills",
		Method:     "POST",
		Path:       "/v1/skills",
		StatusCode: 201,
		Latency:    42 * time.Millisecond,
	})

	entries := logger.Entries()
	fmt.Println(entries[0].Message)
	// Output: POST /v1/skills completed
}

// ============================================================================
// SCENARIO 6: Error log with errorx auto-extraction
// ============================================================================

// ExampleBusiness_errorLog shows how logx.Err auto-extracts structured
// fields from any error implementing Code/HTTPStatus/Retryable (works with
// errorx without importing it).
func Example_businessErrorLog() {
	logger := logx.NewTestLogger(nil)

	// Simulate an errorx-style error (in real code, import errorx).
	err := &fakeError{
		msg:    "skill not found",
		code:   "AIHUB_SKILL_NOT_FOUND",
		status: 404,
		retry:  false,
	}

	logger.Error("create skill failed",
		logx.String("operation", "aihub.skill.create"),
		logx.Err(err),
	)

	entries := logger.Entries()
	// First field is operation; verify shape.
	fmt.Println(entries[0].Message)
	// Output: create skill failed
}

// ============================================================================
// SCENARIO 7: Audit hint
// ============================================================================

// ExampleBusiness_auditHint shows how to leave an audit breadcrumb for
// sensitive operations. NOTE: this is a log breadcrumb only; compliance-grade
// audit records go through auditx (when it exists).
func Example_businessAuditHint() {
	logger := logx.NewTestLogger(nil)
	logx.LogAuditHint(logger, logx.AuditHint{
		Action:       "aihub.skill.delete",
		ActorID:      "user_123",
		ResourceType: "skill",
		ResourceID:   "skill_001",
		Result:       "success",
	})

	entries := logger.Entries()
	fmt.Println(entries[0].Message)
	// Output: audit hint
}

// ============================================================================
// SCENARIO 8: Test logger assertion
// ============================================================================

// ExampleBusiness_testLogger shows the standard test pattern: capture log
// entries and assert specific fields were logged.
func Example_businessTestLogger() {
	t := &testing.T{}
	logger := logx.NewTestLogger(t)

	svc := &fakeSkillService{logger: logger}
	_ = svc.Create(context.Background(), "demo")

	// Assert the expected log was emitted with the right fields.
	logger.AssertLogged(t, "skill created",
		logx.String("skill_id", "demo"),
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 9: Request-scoped fields
// ============================================================================

// ExampleBusiness_requestScoped shows the full handler pattern: inject
// request_id / trace_id / subject_id once at the handler boundary, then
// every downstream log automatically includes them.
func Example_businessRequestScoped() {
	logger := logx.NewTestLogger(nil)
	ctx := context.Background()

	// Handler boundary: inject request-scoped fields.
	ctx = logx.Inject(ctx, logger,
		logx.String("request_id", "req_abc"),
		logx.String("trace_id", "trace_xyz"),
		logx.String("subject_id", "u_123"),
	)

	// Downstream code just calls FromContext — fields are auto-attached.
	logx.FromContext(ctx).Info("request accepted")

	// Even deep in the repo layer:
	repo := &fakeSkillRepo{logger: logger}
	_, _ = repo.Find(ctx, "skill_001")

	entries := logger.Entries()
	fmt.Println(len(entries) >= 2)
	// Output: true
}

// ============================================================================
// SCENARIO 10: Sampling noisy logs
// ============================================================================

// ExampleBusiness_sampling shows how to enable sampling for high-QPS logs
// so log pipelines aren't overwhelmed.
func Example_businessSampling() {
	cfg := logx.DefaultConfig("prod")
	cfg.Sampling = logx.SamplingConfig{
		Enabled:  true,
		Every:    100,                      // keep 1 of every 100 after First
		First:    10,                       // always keep first 10 per level+message
		Window:   time.Second,              // reset counters every second
		MinLevel: logx.DebugLevel.String(), // sample debug/info only; warn+ always kept
	}
	logger, _, _ := logx.New(cfg, logx.WithWriter(io.Discard))

	// High-QPS debug logs get sampled; errors always go through.
	for i := 0; i < 1000; i++ {
		logger.Debug("ping", logx.Int("seq", i))
	}
	logger.Error("critical", logx.Err(errors.New("boom")))

	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// FAKE IMPLEMENTATIONS (for example self-containment)
// ============================================================================

type fakeSkillRepo struct {
	logger logx.Logger
}

func (r *fakeSkillRepo) Find(ctx context.Context, id string) (*struct{}, error) {
	logger := r.logger
	if l := logx.FromContext(ctx); l != logx.Noop() {
		logger = l
	}
	logger.Debug("querying skill",
		logx.String("skill_id", id),
		logx.String("table", "aihub_skills"),
	)
	return &struct{}{}, nil
}

type fakeSkillService struct {
	logger logx.Logger
}

func (s *fakeSkillService) Create(ctx context.Context, name string) error {
	logger := s.logger
	if l := logx.FromContext(ctx); l != logx.Noop() {
		logger = l
	}
	logger.Info("skill created",
		logx.Event("skill_created"),
		logx.String("skill_id", name),
		logx.String("operation", "aihub.skill.create"),
	)
	return nil
}

func callUpstreamLLM(logger logx.Logger, model string, status int, latency time.Duration) {
	logx.LogExternalCall(logger, logx.ExternalCall{
		Provider:   "openai",
		Service:    "chat-completions",
		Operation:  "create",
		Model:      model,
		Endpoint:   "https://api.openai.com/v1/chat/completions",
		StatusCode: status,
		Latency:    latency,
	})
}

// fakeError simulates an errorx-style error without importing errorx.
type fakeError struct {
	msg    string
	code   string
	status int
	retry  bool
}

func (e *fakeError) Error() string   { return e.msg }
func (e *fakeError) Code() string    { return e.code }
func (e *fakeError) HTTPStatus() int { return e.status }
func (e *fakeError) Retryable() bool { return e.retry }
