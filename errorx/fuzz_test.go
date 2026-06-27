package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func FuzzNewError(f *testing.F) {
	f.Add("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	f.Add("", "")
	f.Add("bad-code", "message")

	f.Fuzz(func(t *testing.T, code string, message string) {
		err := errorx.New(errorx.Code(code), errorx.WithMessage(message))
		_ = err.Error()
		_ = errorx.CodeOf(err)
		_ = errorx.MessageOf(err)
		_ = errorx.HTTPStatusOf(err)
		_ = errorx.GRPCCodeOf(err)
		_ = errorx.RetryableOf(err)
		_ = errorx.MetadataOf(err)
		_ = errorx.Fields(err)
		_ = errorx.MetricsLabels(err)
	})
}
