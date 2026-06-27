package errorx_test

import (
	"fmt"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func BenchmarkCodeOf(b *testing.B) {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errorx.CodeOf(err)
	}
}

func BenchmarkHTTPStatusOfWrapped(b *testing.B) {
	err := fmt.Errorf("repo: %w", errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在"))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errorx.HTTPStatusOf(err)
	}
}

func BenchmarkFields(b *testing.B) {
	err := errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
		errorx.WithMetadata("model", "qwen3"),
		errorx.WithRequestID("req_123"),
	)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errorx.Fields(err)
	}
}
