package casdoor

import (
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/logx"
)

func errInvalidConfig(message string) error { return authn.ErrInvalidTokenRequest(message) }

func wrapBackend(message string, err error) error {
	if err != nil {
		message = message + ": " + err.Error()
	}
	out := authn.ErrIdentityBackendFailed(message, err)
	logger := logx.DefaultLogger().Named("authn.casdoor")
	if err != nil {
		logger.Error("casdoor backend failed", logx.String("operation", message), logx.String("cause", err.Error()), logx.Err(out))
	} else {
		logger.Error("casdoor backend failed", logx.String("operation", message), logx.Err(out))
	}
	return out
}
