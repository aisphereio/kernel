package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestIsValidCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code errorx.Code
		want bool
	}{
		{name: "ok", code: errorx.CodeOK, want: true},
		{name: "business code", code: "AIHUB_SKILL_NOT_FOUND", want: true},
		{name: "number suffix", code: "MODEL_PROVIDER_V2_TIMEOUT", want: true},
		{name: "empty", code: "", want: false},
		{name: "lower", code: "skill_not_found", want: false},
		{name: "kebab", code: "SKILL-NOT-FOUND", want: false},
		{name: "space", code: "SKILL NOT FOUND", want: false},
		{name: "starts with digit", code: "1_SKILL_NOT_FOUND", want: false},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := errorx.IsValidCode(tt.code); got != tt.want {
				t.Fatalf("IsValidCode(%q)=%v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestNormalizeCode(t *testing.T) {
	t.Parallel()

	if got := errorx.NormalizeCode(""); got != errorx.CodeInternal {
		t.Fatalf("NormalizeCode(empty)=%q, want %q", got, errorx.CodeInternal)
	}
	if got := errorx.NormalizeCode("AIHUB_SKILL_NOT_FOUND"); got != "AIHUB_SKILL_NOT_FOUND" {
		t.Fatalf("NormalizeCode(custom)=%q", got)
	}
}

func TestValidateCode(t *testing.T) {
	t.Parallel()

	if err := errorx.ValidateCode("AIHUB_SKILL_NOT_FOUND"); err != nil {
		t.Fatalf("ValidateCode(valid) returned error: %v", err)
	}

	err := errorx.ValidateCode("skill-not-found")
	if err == nil {
		t.Fatal("ValidateCode(invalid) returned nil")
	}
	if got := errorx.CodeOf(err); got != errorx.CodeBadRequest {
		t.Fatalf("ValidateCode(invalid) code=%q, want %q", got, errorx.CodeBadRequest)
	}
	if got := errorx.MetadataOf(err)["code"]; got != "skill-not-found" {
		t.Fatalf("ValidateCode(invalid) metadata code=%v", got)
	}
}

func TestMustCode(t *testing.T) {
	t.Parallel()

	if got := errorx.MustCode("AIHUB_SKILL_NOT_FOUND"); got != "AIHUB_SKILL_NOT_FOUND" {
		t.Fatalf("MustCode(valid)=%q", got)
	}

	assertPanic(t, func() {
		_ = errorx.MustCode("skill-not-found")
	})
}

func TestMustValidCodes(t *testing.T) {
	t.Parallel()

	errorx.MustValidCodes(errorx.CodeOK, "AIHUB_SKILL_NOT_FOUND", "MODEL_PROVIDER_V2_TIMEOUT")

	assertPanic(t, func() {
		errorx.MustValidCodes(errorx.CodeOK, "skill-not-found")
	})
}
