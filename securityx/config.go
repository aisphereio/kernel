// Package securityx groups external configuration for authn, authz and auditx.
//
// It intentionally does not hide the underlying implementations. Apps can scan
// a unified YAML tree into Config, then construct authn/casdoor,
// authz/spicedb and auditx stores explicitly during boot.
package securityx

import (
	"github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/configx"
)

const DefaultConfigKey = "security"

type Config struct {
	Authn AuthnConfig `json:"authn" yaml:"authn"`
	Authz AuthzConfig `json:"authz" yaml:"authz"`
	Audit AuditConfig `json:"audit" yaml:"audit"`
}

type AuthnConfig struct {
	Provider string         `json:"provider" yaml:"provider"`
	Casdoor  casdoor.Config `json:"casdoor" yaml:"casdoor"`
}

type AuthzConfig struct {
	Provider string         `json:"provider" yaml:"provider"`
	SpiceDB  spicedb.Config `json:"spicedb" yaml:"spicedb"`
}

type AuditConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Backend string `json:"backend" yaml:"backend"`
	DSN     string `json:"dsn" yaml:"dsn"`
}

func (c Config) Normalized() Config {
	if c.Authn.Provider == "" {
		c.Authn.Provider = "casdoor"
	}
	if c.Authz.Provider == "" {
		c.Authz.Provider = "spicedb"
	}
	c.Authn.Casdoor = c.Authn.Casdoor.Normalized()
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
