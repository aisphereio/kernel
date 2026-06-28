package casdoor

import (
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/logx"
)

func errInvalidConfig(message string) error { return authn.ErrInvalidTokenRequest(message) }

func wrapBackend(message string, err error) error {
	out := authn.ErrIdentityBackendFailed(message, err)
	logx.DefaultLogger().Named("authn.casdoor").Error("casdoor backend failed", logx.String("operation", message), logx.Err(out))
	return out
}
