package errorx_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aisphereio/kernel/errorx"
)

// This file demonstrates COMPLETE business scenarios showing how errorx
// fits into handler → service → repository flow. AI tools should copy these
// patterns when generating new business code.
//
// Each Example here is a self-contained scenario that can be adapted to any
// domain (Skill / Agent / Workflow / Tool / ...).

// ============================================================================
// SCENARIO 1: Repository layer — convert DB errors to errorx
// ============================================================================

// ExampleBusiness_repositoryLayer shows the standard pattern for converting
// database errors (sql.ErrNoRows, connection errors) to errorx errors.
func Example_businessRepositoryLayer() {
	repo := &fakeSkillRepo{}
	_, err := repo.Find(context.Background(), "missing")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	// Output:
	// AIHUB_SKILL_NOT_FOUND
	// 404
}

// ============================================================================
// SCENARIO 2: Service layer — validation + authz + business rules
// ============================================================================

// ExampleBusiness_serviceLayer shows the standard pattern for service-layer
// validation, permission checks, and conflict detection — all returning errorx.
func Example_businessServiceLayer() {
	svc := &fakeSkillService{}
	_, err := svc.Create(context.Background(), "")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))

	_, err = svc.Create(context.Background(), "demo")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	// Output:
	// AIHUB_SKILL_NAME_REQUIRED
	// 400
	// AIHUB_SKILL_ALREADY_EXISTS
	// 409
}

// ============================================================================
// SCENARIO 3: Upstream dependency failure (model API timeout)
// ============================================================================

// ExampleBusiness_upstreamTimeout shows how to wrap upstream API failures
// with retryable flag so the worker can decide whether to retry.
func Example_businessUpstreamTimeout() {
	err := callModelService(context.Background(), "gpt-4")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.RetryableOf(err))
	fmt.Println(errorx.CategoryOf(err))
	// Output:
	// MODEL_UPSTREAM_TIMEOUT
	// 504
	// true
	// dependency
}

// ============================================================================
// SCENARIO 4: Authz denied
// ============================================================================

// ExampleBusiness_authzDenied shows the standard pattern for permission denied,
// including the resource / action in metadata for audit.
func Example_businessAuthzDenied() {
	err := checkPermission(context.Background(), "user_123", "aihub:skill:demo", "skill.delete")
	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.MetadataOf(err)["subject_id"])
	fmt.Println(errorx.PublicMetadataOf(err)["resource"])
	// Output:
	// AIHUB_SKILL_DELETE_DENIED
	// 403
	// user_123
	// skill
}

// ============================================================================
// SCENARIO 5: Worker retry decision
// ============================================================================

// ExampleBusiness_workerRetry shows how a worker uses RetryableOf to decide
// whether to retry a failed job.
func Example_businessWorkerRetry() {
	cases := []struct {
		name string
		err  error
	}{
		{"db_timeout", errorx.Wrap(errors.New("pq: timeout"), "AIHUB_DB_TIMEOUT",
			errorx.WithRetryable(true))},
		{"validation", errorx.BadRequest("AIHUB_INVALID_INPUT", "bad input")},
		{"upstream_down", errorx.Unavailable("MODEL_UNAVAILABLE", "model down")},
	}

	for _, c := range cases {
		action := "fail"
		if errorx.RetryableOf(c.err) {
			action = "retry"
		}
		fmt.Printf("%s -> %s\n", c.name, action)
	}
	// Output:
	// db_timeout -> retry
	// validation -> fail
	// upstream_down -> retry
}

// ============================================================================
// SCENARIO 6: HTTP handler error → JSON response shape
// ============================================================================

// ExampleBusiness_httpResponse shows the JSON shape that httpx middleware
// produces from an errorx error. The middleware extracts code/message/
// request_id/trace_id/public_metadata automatically.
func Example_businessHTTPResponse() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithPublicMetadata("resource", "skill"),
		errorx.WithRequestID("req_abc"),
	)

	// In real code, httpx middleware does this:
	resp := map[string]any{
		"code":       errorx.CodeOf(err).String(),
		"message":    errorx.MessageOf(err),
		"request_id": errorx.RequestIDOf(err),
		"metadata":   errorx.PublicMetadataOf(err),
	}
	fmt.Println(resp["code"])
	fmt.Println(resp["message"])
	fmt.Println(resp["request_id"])
	fmt.Println(resp["metadata"])
	// Output:
	// AIHUB_SKILL_NOT_FOUND
	// 技能不存在
	// req_abc
	// map[resource:skill]
}

// ============================================================================
// SCENARIO 7: Audit record from error
// ============================================================================

// ExampleBusiness_auditRecord shows how auditx extracts error fields for
// audit log entries.
func Example_businessAuditRecord() {
	err := errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "权限被拒绝",
		errorx.WithMetadata("subject_id", "user_123"),
		errorx.WithMetadata("resource", "aihub:skill:demo"),
		errorx.WithRequestID("req_xyz"),
	)

	auditRecord := map[string]string{
		"action":        "aihub.skill.delete",
		"result":        "deny",
		"error_code":    errorx.CodeOf(err).String(),
		"error_message": errorx.MessageOf(err),
		"request_id":    errorx.RequestIDOf(err),
	}
	fmt.Println(auditRecord["result"])
	fmt.Println(auditRecord["error_code"])
	// Output:
	// deny
	// AIHUB_SKILL_DELETE_DENIED
}

// ============================================================================
// SCENARIO 8: Log entry from error (with redacted metadata)
// ============================================================================

// ExampleBusiness_logEntry shows how logx extracts error fields for
// structured log entries. Note that SafeMetadataOf redacts sensitive keys.
func Example_businessLogEntry() {
	err := errorx.Internal("AIHUB_DB_FAILED", "数据库失败",
		errorx.WithMetadata("password", "s3cr3t"),
		errorx.WithMetadata("dsn", "postgres://..."),
		errorx.WithMetadata("query", "SELECT 1"),
		errorx.WithRequestID("req_123"),
	)

	logFields := errorx.Fields(err)
	fmt.Println(logFields["error_code"])
	fmt.Println(logFields["http_status"])
	fmt.Println(logFields["severity"])
	// SafeMetadataOf is used internally by Fields for "error_metadata":
	safeMD := logFields["error_metadata"].(map[string]any)
	fmt.Println(safeMD["password"]) // redacted
	fmt.Println(safeMD["dsn"])      // not redacted (no sensitive keyword)
	// Output:
	// AIHUB_DB_FAILED
	// 500
	// error
	// [REDACTED]
	// postgres://...
}

// ============================================================================
// SCENARIO 9: Metrics labels (low cardinality)
// ============================================================================

// ExampleBusiness_metricsLabels shows what metricsx emits for an error.
// Note: ONLY low-cardinality fields, NEVER dynamic values.
func Example_businessMetricsLabels() {
	err := errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型超时",
		errorx.WithMetadata("model", "gpt-4"),   // dynamic — NOT in labels
		errorx.WithMetadata("user_id", "u_123"), // dynamic — NOT in labels
		errorx.WithRequestID("req_abc"),         // dynamic — NOT in labels
	)

	labels := errorx.MetricsLabels(err)
	for _, k := range []string{"error_code", "http_status", "grpc_code", "retryable", "category"} {
		fmt.Printf("%s=%s\n", k, labels[k])
	}
	// Output:
	// error_code=MODEL_UPSTREAM_TIMEOUT
	// http_status=504
	// grpc_code=DeadlineExceeded
	// retryable=true
	// category=dependency
}

// ============================================================================
// SCENARIO 10: Multi-layer wrap (preserve full chain)
// ============================================================================

// ExampleBusiness_multiLayerWrap shows how errors propagate through layers
// while preserving the original cause for errors.Is.
func Example_businessMultiLayerWrap() {
	// Layer 1: database driver
	dbErr := errors.New("pq: connection refused")

	// Layer 2: repository wraps with business code
	repoErr := errorx.Wrap(dbErr, "AIHUB_SKILL_QUERY_FAILED",
		errorx.WithMessage("查询技能失败"),
		errorx.WithRetryable(true),
		errorx.WithMetadata("skill_id", "skill_001"),
	)

	// Layer 3: service wraps again (less common, but possible)
	svcErr := errorx.Wrap(repoErr, "AIHUB_SKILL_GET_FAILED",
		errorx.WithMessage("获取技能失败"),
		errorx.WithRetryable(errorx.RetryableOf(repoErr)),
	)

	// errors.Is walks the entire chain back to the original dbErr.
	fmt.Println(errors.Is(svcErr, dbErr))
	fmt.Println(errorx.CodeOf(svcErr))
	fmt.Println(errorx.RetryableOf(svcErr))
	// Output:
	// true
	// AIHUB_SKILL_GET_FAILED
	// true
}

// ============================================================================
// FAKE IMPLEMENTATIONS (for example self-containment)
// ============================================================================

type fakeSkillRepo struct{}

func (r *fakeSkillRepo) Find(ctx context.Context, id string) (*struct{}, error) {
	// Simulate database not-found via sql.ErrNoRows.
	err := sql.ErrNoRows
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
			errorx.WithMetadata("skill_id", id),
			errorx.WithPublicMetadata("resource", "skill"),
		)
	}
	return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
		errorx.WithMessage("查询技能失败"),
		errorx.WithRetryable(true),
		errorx.WithMetadata("skill_id", id),
	)
}

type fakeSkillService struct{}

func (s *fakeSkillService) Create(ctx context.Context, name string) (*struct{}, error) {
	if name == "" {
		return nil, errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "技能名称不能为空")
	}
	if name == "demo" {
		return nil, errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在",
			errorx.WithPublicMetadata("name", name),
		)
	}
	return &struct{}{}, nil
}

func callModelService(ctx context.Context, model string) error {
	return errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
		errorx.WithRetryable(true),
		errorx.WithMetadata("model", model),
		errorx.WithPublicMetadata("model", model),
	)
}

func checkPermission(ctx context.Context, subjectID, resource, action string) error {
	return errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除技能的权限",
		errorx.WithMetadata("subject_id", subjectID),
		errorx.WithMetadata("resource", resource),
		errorx.WithMetadata("action", action),
		errorx.WithPublicMetadata("resource", "skill"),
	)
}
