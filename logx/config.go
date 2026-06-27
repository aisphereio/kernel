package logx

import "time"

type Output string

const (
	OutputStdout Output = "stdout"
	OutputStderr Output = "stderr"
)

type Config struct {
	ServiceName string          `json:"service_name" yaml:"service_name"`
	Env         string          `json:"env" yaml:"env"`
	Version     string          `json:"version" yaml:"version"`
	NodeID      string          `json:"node_id" yaml:"node_id"`
	Level       string          `json:"level" yaml:"level"`
	Format      Format          `json:"format" yaml:"format"`
	Output      string          `json:"output" yaml:"output"` // stdout, stderr, or file path
	AddSource   bool            `json:"add_source" yaml:"add_source"`
	Redact      RedactConfig    `json:"redact" yaml:"redact"`
	Sampling    SamplingConfig  `json:"sampling" yaml:"sampling"`
	AccessLog   AccessLogConfig `json:"access_log" yaml:"access_log"`
}

type RedactConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Keys    []string `json:"keys" yaml:"keys"`
	Value   string   `json:"value" yaml:"value"`
}

type SamplingConfig struct {
	Enabled  bool          `json:"enabled" yaml:"enabled"`
	Every    int           `json:"every" yaml:"every"`         // keep 1 of every N records after First
	First    int           `json:"first" yaml:"first"`         // always keep first N records per level/message key
	Window   time.Duration `json:"window" yaml:"window"`       // reset sampling counters periodically
	MinLevel string        `json:"min_level" yaml:"min_level"` // only sample levels <= min_level, default debug
}

type AccessLogConfig struct {
	Enabled         bool          `json:"enabled" yaml:"enabled"`
	SkipPaths       []string      `json:"skip_paths" yaml:"skip_paths"`
	SlowThreshold   time.Duration `json:"slow_threshold" yaml:"slow_threshold"`
	RequestIDHeader string        `json:"request_id_header" yaml:"request_id_header"`
	RouteHeader     string        `json:"route_header" yaml:"route_header"`
	LogUserAgent    bool          `json:"log_user_agent" yaml:"log_user_agent"`
	LogReferer      bool          `json:"log_referer" yaml:"log_referer"`
}

func DefaultConfig(env string) Config {
	if env == "" {
		env = "dev"
	}
	format := FormatJSON
	if env == "dev" || env == "local" {
		format = FormatConsole
	}
	return Config{
		ServiceName: "app",
		Env:         env,
		Version:     "dev",
		Level:       InfoLevel.String(),
		Format:      format,
		Output:      string(OutputStdout),
		AddSource:   env == "dev" || env == "local",
		Redact: RedactConfig{
			Enabled: true,
			Keys:    DefaultRedactKeys(),
			Value:   "***",
		},
		Sampling: SamplingConfig{
			Enabled:  false,
			Every:    100,
			First:    100,
			Window:   time.Second,
			MinLevel: DebugLevel.String(),
		},
		AccessLog: AccessLogConfig{
			Enabled:         true,
			SkipPaths:       []string{"/healthz", "/readyz", "/metrics"},
			SlowThreshold:   time.Second,
			RequestIDHeader: "X-Request-ID",
			RouteHeader:     "X-Route-Pattern",
			LogUserAgent:    true,
			LogReferer:      false,
		},
	}
}
