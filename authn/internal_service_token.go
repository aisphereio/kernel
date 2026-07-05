package authn

import (
	"crypto/subtle"
	"strings"
)

const (
	// InternalServiceTokenHeader is the default Gateway -> backend service
	// shared-secret header. It proves the request crossed the trusted Gateway
	// boundary; it does not identify the end user. User identity is still carried
	// by the Gateway-injected X-Aisphere-* Principal headers.
	InternalServiceTokenHeader = "X-Aisphere-Internal-Token"
)

// InternalServiceTokenConfig configures the simple first-phase Gateway ->
// backend trust boundary.
//
// It is intentionally a shared-secret mechanism for MVP deployments. Production
// can later add NetworkPolicy and mTLS without changing the authn/accessx
// contract.
type InternalServiceTokenConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	HeaderName string `json:"header" yaml:"header"`
	Token      string `json:"token" yaml:"token"`
}

func (c InternalServiceTokenConfig) Normalized() InternalServiceTokenConfig {
	c.HeaderName = strings.TrimSpace(c.HeaderName)
	if c.HeaderName == "" {
		c.HeaderName = InternalServiceTokenHeader
	}
	c.Token = strings.TrimSpace(c.Token)
	return c
}

func (c InternalServiceTokenConfig) Header() string { return c.Normalized().HeaderName }

func (c InternalServiceTokenConfig) ValidateToken(got string) bool {
	c = c.Normalized()
	if !c.Enabled {
		return true
	}
	if c.Token == "" || strings.TrimSpace(got) == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Token), []byte(strings.TrimSpace(got))) == 1
}

// StripInternalServiceToken removes the Gateway-to-backend shared secret header.
// Gateways must call this before injecting their own value so clients cannot
// smuggle a forged internal token through the edge.
func StripInternalServiceToken(headers map[string]string, headerName string) {
	if headers == nil {
		return
	}
	headerName = strings.TrimSpace(headerName)
	if headerName == "" {
		headerName = InternalServiceTokenHeader
	}
	for existing := range headers {
		if strings.EqualFold(existing, headerName) || strings.EqualFold(existing, InternalServiceTokenHeader) {
			delete(headers, existing)
		}
	}
}

// InjectInternalServiceToken injects the configured Gateway-to-backend token.
func InjectInternalServiceToken(headers map[string]string, cfg InternalServiceTokenConfig) {
	if headers == nil {
		return
	}
	cfg = cfg.Normalized()
	if !cfg.Enabled || cfg.Token == "" {
		return
	}
	headers[cfg.HeaderName] = cfg.Token
}

func internalServiceTokenFromMetadata(metadata map[string]string, headerName string) string {
	if metadata == nil {
		return ""
	}
	headerName = strings.TrimSpace(headerName)
	if headerName == "" {
		headerName = InternalServiceTokenHeader
	}
	for k, v := range metadata {
		if strings.EqualFold(k, headerName) || strings.EqualFold(k, InternalServiceTokenHeader) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
