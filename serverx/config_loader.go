package serverx

import (
	"fmt"
	"time"

	"github.com/aisphereio/kernel/configx"
	configfile "github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/migrationx"
)

// FileConfig is the YAML/JSON friendly service configuration contract used by
// Kernel-generated services. It intentionally mirrors Config, but keeps duration
// fields as strings so humans can write values like "2s" in config files.
type FileConfig struct {
	App struct {
		Name    string `json:"name" yaml:"name"`
		Version string `json:"version" yaml:"version"`
	} `json:"app" yaml:"app"`

	Deployment struct {
		Replicas int `json:"replicas" yaml:"replicas"`
	} `json:"deployment" yaml:"deployment"`

	Server struct {
		HTTP struct {
			Enabled bool   `json:"enabled" yaml:"enabled"`
			Address string `json:"address" yaml:"address"`
			Timeout string `json:"timeout" yaml:"timeout"`
		} `json:"http" yaml:"http"`
		GRPC struct {
			Enabled bool   `json:"enabled" yaml:"enabled"`
			Address string `json:"address" yaml:"address"`
			Timeout string `json:"timeout" yaml:"timeout"`
		} `json:"grpc" yaml:"grpc"`
	} `json:"server" yaml:"server"`

	SystemRoutes SystemRoutesFileConfig `json:"system_routes" yaml:"system_routes"`

	Database struct {
		Enabled            bool              `json:"enabled" yaml:"enabled"`
		Driver             string            `json:"driver" yaml:"driver"`
		DSN                string            `json:"dsn" yaml:"dsn"`
		AutoCreateDatabase bool              `json:"auto_create_database" yaml:"auto_create_database"`
		MaxOpenConns       int               `json:"max_open_conns" yaml:"max_open_conns"`
		MaxIdleConns       int               `json:"max_idle_conns" yaml:"max_idle_conns"`
		QueryTimeout       string            `json:"query_timeout" yaml:"query_timeout"`
		SlowQueryThreshold string            `json:"slow_query_threshold" yaml:"slow_query_threshold"`
		Debug              bool              `json:"debug" yaml:"debug"`
		DryRun             bool              `json:"dry_run" yaml:"dry_run"`
		Migration          migrationx.Config `json:"migration" yaml:"migration"`
	} `json:"database" yaml:"database"`

	Security struct {
		Authn struct {
			Provider string `json:"provider" yaml:"provider"`
			Required bool   `json:"required" yaml:"required"`
		} `json:"authn" yaml:"authn"`
		Authz struct {
			Provider string `json:"provider" yaml:"provider"`
			Required bool   `json:"required" yaml:"required"`
		} `json:"authz" yaml:"authz"`
		Audit struct {
			Enabled  bool   `json:"enabled" yaml:"enabled"`
			Provider string `json:"provider" yaml:"provider"`
		} `json:"audit" yaml:"audit"`
	} `json:"security" yaml:"security"`
}

type SystemRoutesFileConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
	Healthz bool `json:"healthz" yaml:"healthz"`
	Readyz  bool `json:"readyz" yaml:"readyz"`
	Version bool `json:"version" yaml:"version"`
	Metrics bool `json:"metrics" yaml:"metrics"`
}

// LoadConfigFile loads a Kernel service config from YAML/JSON/TOML sources
// supported by configx/file. New services should use this during boot, then pass
// the returned Config to serverx.New.
func LoadConfigFile(path string) (Config, error) {
	cfg := configx.New(configx.WithSource(configfile.NewSource(path)))
	if err := cfg.Load(); err != nil {
		return Config{}, err
	}
	defer cfg.Close()
	return LoadConfig(cfg)
}

// LoadConfig loads serverx.Config from an already initialized configx.Config.
func LoadConfig(c configx.Config) (Config, error) {
	if c == nil {
		return Config{}, configx.ErrNilConfig
	}
	var fc FileConfig
	if err := c.Scan(&fc); err != nil {
		return Config{}, err
	}
	return DecodeFileConfig(fc)
}

// DecodeFileConfig converts the human-facing FileConfig into the runtime
// Config. It is split out for tests and for services that assemble config from
// multiple sources before booting.
func DecodeFileConfig(fc FileConfig) (Config, error) {
	var out Config
	out.Name = fc.App.Name
	out.Version = fc.App.Version
	out.Deployment.Replicas = fc.Deployment.Replicas
	out.HTTP.Enabled = fc.Server.HTTP.Enabled
	out.HTTP.Address = fc.Server.HTTP.Address
	if fc.Server.HTTP.Timeout != "" {
		d, err := time.ParseDuration(fc.Server.HTTP.Timeout)
		if err != nil {
			return Config{}, fmt.Errorf("server.http.timeout: %w", err)
		}
		out.HTTP.Timeout = d
	}
	out.GRPC.Enabled = fc.Server.GRPC.Enabled
	out.GRPC.Address = fc.Server.GRPC.Address
	if fc.Server.GRPC.Timeout != "" {
		d, err := time.ParseDuration(fc.Server.GRPC.Timeout)
		if err != nil {
			return Config{}, fmt.Errorf("server.grpc.timeout: %w", err)
		}
		out.GRPC.Timeout = d
	}
	out.SystemRoutes = SystemRoutesConfig{
		Enabled: fc.SystemRoutes.Enabled,
		Healthz: fc.SystemRoutes.Healthz,
		Readyz:  fc.SystemRoutes.Readyz,
		Version: fc.SystemRoutes.Version,
		Metrics: fc.SystemRoutes.Metrics,
	}
	out.Security = SecurityConfig{
		Authn: ProviderRefConfig{Provider: fc.Security.Authn.Provider, Required: fc.Security.Authn.Required},
		Authz: ProviderRefConfig{Provider: fc.Security.Authz.Provider, Required: fc.Security.Authz.Required},
		Audit: AuditConfig{Enabled: fc.Security.Audit.Enabled, Provider: fc.Security.Audit.Provider},
	}
	out.Database.Enabled = fc.Database.Enabled
	out.Database.DBX.Driver = fc.Database.Driver
	out.Database.DBX.DSN = fc.Database.DSN
	out.Database.DBX.AutoCreateDatabase = fc.Database.AutoCreateDatabase
	out.Database.DBX.MaxOpenConns = fc.Database.MaxOpenConns
	out.Database.DBX.MaxIdleConns = fc.Database.MaxIdleConns
	out.Database.DBX.Debug = fc.Database.Debug
	out.Database.DBX.DryRun = fc.Database.DryRun
	out.Database.Migration = fc.Database.Migration
	if fc.Database.QueryTimeout != "" {
		d, err := time.ParseDuration(fc.Database.QueryTimeout)
		if err != nil {
			return Config{}, fmt.Errorf("database.query_timeout: %w", err)
		}
		out.Database.DBX.QueryTimeout = d
	}
	if fc.Database.SlowQueryThreshold != "" {
		d, err := time.ParseDuration(fc.Database.SlowQueryThreshold)
		if err != nil {
			return Config{}, fmt.Errorf("database.slow_query_threshold: %w", err)
		}
		out.Database.DBX.SlowQueryThreshold = d
	}
	return normalizeConfig(out), nil
}
