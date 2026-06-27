package errorx_test

import (
	"errors"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestErrorxContract(t *testing.T) {
	t.Parallel()

	t.Run("nil maps to OK", func(t *testing.T) {
		t.Parallel()
		assertEqual(t, errorx.CodeOf(nil), errorx.CodeOK)
		assertEqual(t, errorx.HTTPStatusOf(nil), 200)
		assertEqual(t, errorx.GRPCCodeOf(nil), errorx.GRPCCodeOK)
		assertEqual(t, errorx.RetryableOf(nil), false)
	})

	t.Run("unknown maps to internal", func(t *testing.T) {
		t.Parallel()
		err := errors.New("boom")
		assertEqual(t, errorx.CodeOf(err), errorx.CodeInternal)
		assertEqual(t, errorx.MessageOf(err), "internal server error")
		assertEqual(t, errorx.HTTPStatusOf(err), 500)
		assertEqual(t, errorx.GRPCCodeOf(err), errorx.GRPCCodeInternal)
		assertEqual(t, errorx.RetryableOf(err), false)
	})

	t.Run("wrap preserves cause", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("db timeout")
		err := errorx.Wrap(cause, "AIHUB_SKILL_QUERY_FAILED", errorx.WithMessage("查询技能失败"))
		if !errors.Is(err, cause) {
			t.Fatal("Wrap must preserve cause for errors.Is")
		}
		var kernelErr *errorx.Error
		if !errors.As(err, &kernelErr) {
			t.Fatal("Wrap must preserve *errorx.Error for errors.As")
		}
	})

	t.Run("inspect helpers are safe for any error", func(t *testing.T) {
		t.Parallel()
		for _, err := range []error{nil, errors.New("plain"), errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")} {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("inspect helper panicked for %T: %v", err, r)
					}
				}()
				_ = errorx.CodeOf(err)
				_ = errorx.MessageOf(err)
				_ = errorx.HTTPStatusOf(err)
				_ = errorx.GRPCCodeOf(err)
				_ = errorx.RetryableOf(err)
				_ = errorx.MetadataOf(err)
				_ = errorx.Fields(err)
				_ = errorx.MetricsLabels(err)
			}()
		}
	})

	t.Run("metadata does not break errors.Is or errors.As", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("cause")
		err := errorx.Wrap(cause, "AIHUB_SKILL_CREATE_FAILED",
			errorx.WithMetadata("skill_id", "skill_001"),
			errorx.WithMetadata("project_id", "project_001"),
		)
		if !errors.Is(err, cause) {
			t.Fatal("metadata should not break cause chain")
		}
		var kernelErr *errorx.Error
		if !errors.As(err, &kernelErr) {
			t.Fatal("metadata should not break errors.As")
		}
	})
}
