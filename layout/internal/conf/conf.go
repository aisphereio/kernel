package conf

import (
	"time"

	"github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/cachex"
	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/objectstorex"
)

type Bootstrap struct {
	Service  ServiceConfig  `json:"service" yaml:"service"`
	Server   ServerConfig   `json:"server" yaml:"server"`
	Log      logx.Config    `json:"log" yaml:"log"`
	Data     DataConfig     `json:"data" yaml:"data"`
	Security SecurityConfig `json:"security" yaml:"security"`
	Audit    AuditConfig    `json:"audit" yaml:"audit"`
	Metrics  MetricsConfig  `json:"metrics" yaml:"metrics"`
}

type ServiceConfig struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Env     string `json:"env" yaml:"env"`
}

type ServerConfig struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
	GRPC GRPCConfig `json:"grpc" yaml:"grpc"`
}

type HTTPConfig struct {
	Addr    string        `json:"addr" yaml:"addr"`
	Timeout time.Duration `json:"timeout_ns" yaml:"timeout_ns"`
}

type GRPCConfig struct {
	Addr    string        `json:"addr" yaml:"addr"`
	Timeout time.Duration `json:"timeout_ns" yaml:"timeout_ns"`
}

type DataConfig struct {
	Database    DatabaseConfig    `json:"database" yaml:"database"`
	Cache       CacheConfig       `json:"cache" yaml:"cache"`
	ObjectStore ObjectStoreConfig `json:"object_store" yaml:"object_store"`
}

type DatabaseConfig struct {
	Enabled bool       `json:"enabled" yaml:"enabled"`
	Config  dbx.Config `json:"config" yaml:"config"`
}

type CacheConfig struct {
	Enabled bool          `json:"enabled" yaml:"enabled"`
	Config  cachex.Config `json:"config" yaml:"config"`
}

type ObjectStoreConfig struct {
	Enabled bool                `json:"enabled" yaml:"enabled"`
	Config  objectstorex.Config `json:"config" yaml:"config"`
}

type SecurityConfig struct {
	Authn AuthnConfig `json:"authn" yaml:"authn"`
	Authz AuthzConfig `json:"authz" yaml:"authz"`
}

type AuthnConfig struct {
	Enabled  bool           `json:"enabled" yaml:"enabled"`
	Provider string         `json:"provider" yaml:"provider"`
	Casdoor  casdoor.Config `json:"casdoor" yaml:"casdoor"`
}

type AuthzConfig struct {
	Enabled     bool           `json:"enabled" yaml:"enabled"`
	Provider    string         `json:"provider" yaml:"provider"`
	DevAllowAll bool           `json:"dev_allow_all" yaml:"dev_allow_all"`
	SpiceDB     spicedb.Config `json:"spicedb" yaml:"spicedb"`
}

type AuditConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Store   string `json:"store" yaml:"store"`
}

type MetricsConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}
