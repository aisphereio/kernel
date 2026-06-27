package errorx_test

import (
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

func TestMetadataOptions(t *testing.T) {
	t.Parallel()

	err := errorx.BadRequest("AIHUB_SKILL_INVALID", "参数错误",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithMetadata("", "ignored"),
		errorx.WithMetadataMap(map[string]any{"project_id": "project_001", "tenant_id": "tenant_001"}),
		errorx.WithPublicMetadata("field", "name"),
		errorx.WithPublicMetadataMap(map[string]any{"reason": "required"}),
	)

	md := errorx.MetadataOf(err)
	assertMapValue(t, md, "skill_id", "skill_001")
	assertMapValue(t, md, "project_id", "project_001")
	assertMapValue(t, md, "tenant_id", "tenant_001")
	assertAbsent(t, md, "")

	public := errorx.PublicMetadataOf(err)
	assertMapValue(t, public, "field", "name")
	assertMapValue(t, public, "reason", "required")
}

func TestSafeMetadataRedactsSecrets(t *testing.T) {
	t.Parallel()

	err := errorx.Internal("AIHUB_INTERNAL_FAILED", "内部错误",
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithMetadata("password", "123456"),
		errorx.WithMetadata("access_token", "token-value"),
		errorx.WithMetadata("Authorization", "Bearer abc"),
		errorx.WithMetadata("api_key", "ak"),
	)

	safe := errorx.SafeMetadataOf(err)
	assertMapValue(t, safe, "skill_id", "skill_001")
	assertMapValue(t, safe, "password", errorx.Redacted)
	assertMapValue(t, safe, "access_token", errorx.Redacted)
	assertMapValue(t, safe, "Authorization", errorx.Redacted)
	assertMapValue(t, safe, "api_key", errorx.Redacted)

	raw := errorx.MetadataOf(err)
	assertMapValue(t, raw, "password", "123456")
}

func TestPublicMetadataRedactsSecrets(t *testing.T) {
	t.Parallel()

	err := errorx.BadRequest("AIHUB_SKILL_INVALID", "参数错误",
		errorx.WithPublicMetadata("field", "name"),
		errorx.WithPublicMetadata("access_token", "token-value"),
		errorx.WithPublicMetadataMap(map[string]any{
			"reason":        "required",
			"client_secret": "secret-value",
		}),
	)

	public := errorx.PublicMetadataOf(err)
	assertMapValue(t, public, "field", "name")
	assertMapValue(t, public, "reason", "required")
	assertMapValue(t, public, "access_token", errorx.Redacted)
	assertMapValue(t, public, "client_secret", errorx.Redacted)
}

func TestChainableWithPublicMetadataRedactsSecrets(t *testing.T) {
	t.Parallel()

	base := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
	next := base.WithPublicMetadata(map[string]any{
		"resource":      "skill",
		"authorization": "Bearer abc",
	})

	assertAbsent(t, errorx.PublicMetadataOf(base), "resource")
	assertMapValue(t, errorx.PublicMetadataOf(next), "resource", "skill")
	assertMapValue(t, errorx.PublicMetadataOf(next), "authorization", errorx.Redacted)
}
