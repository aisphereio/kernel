package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestWithStack(t *testing.T) {
	t.Parallel()

	err := errorx.Internal("AIHUB_INTERNAL_FAILED", "内部错误", errorx.WithStack())
	frames := errorx.StackOf(err)
	if len(frames) == 0 {
		t.Fatal("WithStack should capture at least one frame")
	}
	if frames[0].Function == "" || frames[0].File == "" || frames[0].Line <= 0 {
		t.Fatalf("invalid top frame: %+v", frames[0])
	}
}
