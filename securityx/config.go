// Package securityx groups framework-level security configuration and runtime
// construction for authn, access shortcuts, authz and audit.
//
// securityx is not a middleware assembly layer. serverx/autowire remains the
// only service middleware assembly entrypoint. securityx turns config.yaml into
// provider-neutral runtime values consumed by serverx.RuntimeProviders.
package securityx

import (
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/configx"
)

const DefaultConfigKey = "security"

// Config is the canonical security subtree shared by Gateway and backend
// services. Business projects may embed this type directly or map their legacy
// config structs into it during boot.
type Config struct {
	// Authn describes how inbound callers become authn.Principal values. It is
	// consumed by NewRuntime to build AuthnBoundaryRuntime.
	Authn AuthnBoundaryConfig `json:"authn" yaml:"authn"`

	// InternalCall is the Gateway -> backend trust-boundary secret. It is kept as
	// a top-level field because existing configs use security.internal_call. When
	// Authn.InternalCall is empty, NewRuntime copies this value into Authn.
	InternalCall authn.InternalServiceTokenConfig `json:"internal_call" yaml:"internal_call"`

	// Access controls per-operation short-circuits such as public endpoints and
	// bootstrap/self-service endpoints that skip SpiceDB but still require authn.
	Access accessx.AccessConfig `json:"access" yaml:"access"`

	Authz AuthzConfig `json:"authz" yaml:"authz"`
	Audit AuditConfig `json:"audit" yaml:"audit"`
}

// AuthnConfig is kept as a type alias for older code that imported
// securityx.AuthnConfig.
//
// Deprecated: use AuthnBoundaryConfig. This alias remains only as a short-term
// migration shim and must not be used by new generated code or examples.
type AuthnConfig = AuthnBoundaryConfig

type AuthzConfig struct {
	Enabled     bool           `json:"enabled" yaml:"enabled"`
	Provider    string         `json:"provider" yaml:"provider"`
	DevAllowAll bool           `json:"dev_allow_all" yaml:"dev_allow_all"`
	SpiceDB     spicedb.Config `json:"spicedb" yaml:"spicedb"`
}

type AuditConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Backend string `json:"backend" yaml:"backend"`
	DSN     string `json:"dsn" yaml:"dsn"`
}

func (c Config) Normalized() Config {
	// Preserve whether authn.internal_call was explicitly configured before
	// AuthnBoundaryConfig.Normalized fills default header names.
	authnInternalCallUnset := c.Authn.InternalCall.HeaderName == "" && c.Authn.InternalCall.Token == "" && !c.Authn.InternalCall.Enabled
	c.InternalCall = c.InternalCall.Normalized()
	c.Authn = c.Authn.Normalized()
	if authnInternalCallUnset {
		c.Authn.InternalCall = c.InternalCall
	} else {
		c.Authn.InternalCall = c.Authn.InternalCall.Normalized()
	}
	if c.Authz.Provider == "" {
		c.Authz.Provider = "spicedb"
	}
	c.Authz.SpiceDB = c.Authz.SpiceDB.Normalized()
	return c
}

func Scan(cfg configx.Config) (Config, error) {
	return ScanKey(cfg, DefaultConfigKey)
}

func ScanKey(cfg configx.Config, key string) (Config, error) {
	var out Config
	if err := cfg.Value(key).Scan(&out); err != nil {
		return Config{}, err
	}
	return out.Normalized(), nil
}
