package casdoor

import "github.com/aisphereio/kernel/authn"

func errInvalidConfig(message string) error { return authn.ErrInvalidTokenRequest(message) }

func wrapBackend(message string, err error) error {
	return authn.ErrIdentityBackendFailed(message, err)
}
