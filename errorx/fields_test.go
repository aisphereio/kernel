package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestFieldsForLogAndAudit(t *testing.T) {
	t.Parallel()

	err := errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除权限",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithMetadata("token", "secret-token"),
		errorx.WithPublicMetadata("resource", "skill"),
		errorx.WithRequestID("req_123"),
		errorx.WithTraceID("trace_abc"),
	)

	fields := errorx.Fields(err)
	assertMapValue(t, fields, "error_code", "AIHUB_SKILL_DELETE_DENIED")
	assertMapValue(t, fields, "message", "没有删除权限")
	assertMapValue(t, fields, "http_status", 403)
	assertMapValue(t, fields, "grpc_code", errorx.GRPCCodePermissionDenied)
	assertMapValue(t, fields, "retryable", false)
	assertMapValue(t, fields, "category", string(errorx.CategoryPermission))
	assertMapValue(t, fields, "severity", string(errorx.SeverityInfo))
	assertMapValue(t, fields, "request_id", "req_123")
	assertMapValue(t, fields, "trace_id", "trace_abc")
	assertMapValue(t, fields, "error", "没有删除权限")

	md, ok := fields["error_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("error_metadata missing or wrong type: %T", fields["error_metadata"])
	}
	assertMapValue(t, md, "skill_id", "skill_001")
	assertMapValue(t, md, "token", errorx.Redacted)

	public, ok := fields["public_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("public_metadata missing or wrong type: %T", fields["public_metadata"])
	}
	assertMapValue(t, public, "resource", "skill")
}

func TestFieldsForNil(t *testing.T) {
	t.Parallel()

	fields := errorx.Fields(nil)
	assertMapValue(t, fields, "error_code", "OK")
	assertMapValue(t, fields, "message", "success")
	assertMapValue(t, fields, "http_status", 200)
	assertMapValue(t, fields, "grpc_code", errorx.GRPCCodeOK)
	assertMapValue(t, fields, "retryable", false)
	assertMapValue(t, fields, "category", string(errorx.CategoryOK))
	assertMapValue(t, fields, "severity", string(errorx.SeverityInfo))
	assertAbsent(t, fields, "error")
}

func TestMetricsLabelsAreLowCardinality(t *testing.T) {
	t.Parallel()

	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithRequestID("req_123"),
		errorx.WithTraceID("trace_abc"),
	)
	labels := errorx.MetricsLabels(err)

	want := map[string]string{
		"error_code":  "AIHUB_SKILL_NOT_FOUND",
		"http_status": "404",
		"grpc_code":   errorx.GRPCCodeNotFound,
		"retryable":   "false",
		"category":    string(errorx.CategoryNotFound),
	}
	for k, v := range want {
		if labels[k] != v {
			t.Fatalf("labels[%q]=%q, want %q; labels=%v", k, labels[k], v, labels)
		}
	}
	for _, forbidden := range []string{"message", "request_id", "trace_id", "skill_id", "metadata"} {
		if _, ok := labels[forbidden]; ok {
			t.Fatalf("metrics labels should not contain high-cardinality key %q: %v", forbidden, labels)
		}
	}
}
