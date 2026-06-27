package errorx_test

import (
	"encoding/json"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

type httpErrorResponse struct {
	Code           string         `json:"code"`
	Message        string         `json:"message"`
	RequestID      string         `json:"request_id,omitempty"`
	TraceID        string         `json:"trace_id,omitempty"`
	PublicMetadata map[string]any `json:"metadata,omitempty"`
}

func fakeHTTPWriteError(err error) (int, []byte, error) {
	resp := httpErrorResponse{
		Code:           errorx.CodeOf(err).String(),
		Message:        errorx.MessageOf(err),
		RequestID:      errorx.RequestIDOf(err),
		TraceID:        errorx.TraceIDOf(err),
		PublicMetadata: errorx.PublicMetadataOf(err),
	}
	b, marshalErr := json.Marshal(resp)
	return errorx.HTTPStatusOf(err), b, marshalErr
}

func fakeAuditFailure(err error) map[string]string {
	return map[string]string{
		"result":        "fail",
		"reason":        errorx.CodeOf(err).String(),
		"error_code":    errorx.CodeOf(err).String(),
		"error_message": errorx.MessageOf(err),
		"request_id":    errorx.RequestIDOf(err),
		"trace_id":      errorx.TraceIDOf(err),
	}
}

func fakeWorkerDecision(err error) string {
	if errorx.RetryableOf(err) {
		return "retry"
	}
	return "fail"
}

func TestIntegrationHTTPResponseMapping(t *testing.T) {
	t.Parallel()

	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithPublicMetadata("resource", "skill"),
		errorx.WithRequestID("req_123"),
		errorx.WithTraceID("trace_abc"),
	)

	status, body, marshalErr := fakeHTTPWriteError(err)
	if marshalErr != nil {
		t.Fatalf("marshal error response: %v", marshalErr)
	}
	assertEqual(t, status, 404)

	var got httpErrorResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	assertEqual(t, got.Code, "AIHUB_SKILL_NOT_FOUND")
	assertEqual(t, got.Message, "技能不存在")
	assertEqual(t, got.RequestID, "req_123")
	assertEqual(t, got.TraceID, "trace_abc")
	assertMapValue(t, got.PublicMetadata, "resource", "skill")
	assertAbsent(t, got.PublicMetadata, "skill_id")
}

func TestIntegrationLogAuditMetricsWorker(t *testing.T) {
	t.Parallel()

	err := errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
		errorx.WithMetadata("model", "qwen3"),
		errorx.WithRequestID("req_456"),
	)

	logFields := errorx.Fields(err)
	assertMapValue(t, logFields, "error_code", "MODEL_UPSTREAM_TIMEOUT")
	assertMapValue(t, logFields, "http_status", 504)
	assertMapValue(t, logFields, "retryable", true)

	audit := fakeAuditFailure(err)
	assertEqual(t, audit["reason"], "MODEL_UPSTREAM_TIMEOUT")
	assertEqual(t, audit["error_message"], "模型服务超时")
	assertEqual(t, audit["request_id"], "req_456")

	metrics := errorx.MetricsLabels(err)
	assertEqual(t, metrics["error_code"], "MODEL_UPSTREAM_TIMEOUT")
	assertEqual(t, metrics["retryable"], "true")
	if _, ok := metrics["model"]; ok {
		t.Fatalf("metrics labels must not include dynamic metadata: %v", metrics)
	}

	assertEqual(t, fakeWorkerDecision(err), "retry")
	assertEqual(t, fakeWorkerDecision(errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "名称不能为空")), "fail")
}
