package dtmx

import (
	"strings"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

const (
	DriverDTM = "dtm"

	ProtocolHTTP = "http"
)

// Config configures Kernel distributed transactions.
//
// This config is for the application-side transaction manager client. The DTM
// server itself is an external process managed by deployment; kernel only needs
// the DTM API endpoint and service callback address.
type Config struct {
	// Enabled controls whether distributed transactions are active. When false,
	// New returns a disabled no-op Manager that fails fast on SubmitSaga.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Driver selects the registered implementation. The production driver is
	// "dtm" from package github.com/aisphereio/kernel/dtmx/dtm.
	Driver string `json:"driver" yaml:"driver"`

	// Protocol selects how DTM calls branch endpoints. Currently "http" is
	// implemented; gRPC can be added without changing business code.
	Protocol string `json:"protocol" yaml:"protocol"`

	// Server is the DTM HTTP API endpoint, e.g.
	// http://127.0.0.1:36789/api/dtmsvr.
	Server string `json:"server" yaml:"server"`

	// ServiceBaseURL is the externally reachable base URL for this service.
	// Helpers use it to build branch callback URLs.
	ServiceBaseURL string `json:"service_base_url" yaml:"service_base_url"`

	// BranchPrefix is the prefix for internal branch endpoints, e.g.
	// /internal/dtm. It is optional and used only by BranchURL helper.
	BranchPrefix string `json:"branch_prefix" yaml:"branch_prefix"`

	// WaitResult asks DTM to wait for Saga execution result on Submit. Use true
	// in synchronous user-facing flows; false in fire-and-retry background flows.
	WaitResult bool `json:"wait_result" yaml:"wait_result"`

	// Timeout bounds the local SubmitSaga call. DTM itself still owns branch
	// retry/timeout behavior.
	Timeout time.Duration `json:"timeout_ns" yaml:"timeout_ns"`

	// BranchSecret is propagated to DTM as a branch header. DTM then forwards it
	// to branch action/compensate endpoints, where ValidateBranchRequest or
	// BranchAuthMiddleware can verify the callback. Keep it empty only when the
	// branch endpoints are protected by stronger network controls such as mTLS
	// and private networking.
	BranchSecret string `json:"branch_secret" yaml:"branch_secret"`

	// Logger is the component logger. If nil, dtmx uses logx.DefaultLogger().
	Logger logx.Logger `json:"-" yaml:"-"`

	// Metrics is the optional metrics manager.
	Metrics metricsx.Manager `json:"-" yaml:"-"`

	// MetricsEnabled controls whether dtmx records metrics.
	MetricsEnabled bool `json:"metrics_enabled" yaml:"metrics_enabled"`
}

func (c Config) withDefaults() Config {
	if c.Driver == "" {
		c.Driver = DriverDTM
	}
	c.Driver = strings.ToLower(strings.TrimSpace(c.Driver))
	if c.Protocol == "" {
		c.Protocol = ProtocolHTTP
	}
	c.Protocol = strings.ToLower(strings.TrimSpace(c.Protocol))
	c.Server = strings.TrimRight(strings.TrimSpace(c.Server), "/")
	c.ServiceBaseURL = strings.TrimRight(strings.TrimSpace(c.ServiceBaseURL), "/")
	if c.BranchPrefix != "" && !strings.HasPrefix(c.BranchPrefix, "/") {
		c.BranchPrefix = "/" + c.BranchPrefix
	}
	c.BranchPrefix = strings.TrimRight(c.BranchPrefix, "/")
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.Logger == nil {
		c.Logger = logx.DefaultLogger()
	}
	c.Metrics = metricsx.Ensure(c.Metrics)
	return c
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	c = c.withDefaults()
	if c.Driver == "" || c.Server == "" {
		return ErrInvalidConfig
	}
	if c.Protocol != ProtocolHTTP {
		return ErrUnsupportedProtocol
	}
	return nil
}
