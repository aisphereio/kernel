package kubernetesx

import (
	"errors"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

// Config holds the connection configuration for a Kubernetes API server.
//
// A Config is built either from an explicit Host (plus optional Credential
// merged via MergeCredential), from a kubeconfig byte blob, or — when both are
// empty — from the in-cluster service account. Hub callers normally construct
// Config from a stored Credential and never expose kubeconfig to end users.
type Config struct {
	// Host is the API server URL, e.g. "https://10.0.0.1:6443".
	// Required when Kubeconfig is empty and the process is not in-cluster.
	Host string `json:"host"`

	// ServerName overrides the TLS hostname used to verify the API server
	// certificate. Use it when Host is an IP address but the server
	// certificate is signed for a DNS name (e.g. Hub reaches a remote
	// cluster via a public IP while the cluster cert SAN only lists the
	// in-cluster domain). Setting ServerName preserves CA verification —
	// it does NOT disable TLS. Empty means the hostname is derived from
	// Host. Ignored when Kubeconfig is supplied unless the caller also
	// sets it explicitly after MergeCredential.
	ServerName string `json:"server_name"`

	// Kubeconfig is the raw kubeconfig YAML. When non-empty it takes priority
	// over Host for constructing *rest.Config. The selected context is
	// controlled by Context.
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// Context selects the kubeconfig context to use. Empty means the
	// current-context of the kubeconfig.
	Context string `json:"context"`

	// Token is the bearer token used when authenticating with a
	// CredentialKindServiceAccount credential. Populated by MergeCredential;
	// ignored when Kubeconfig is non-empty. Like Kubeconfig, this field
	// carries secret material and must not be logged.
	Token string `json:"token,omitempty"`

	// CACert is the PEM-encoded CA certificate used to verify the API server
	// TLS certificate when authenticating with a service-account credential.
	// Populated by MergeCredential; ignored when Kubeconfig is non-empty.
	CACert []byte `json:"ca_cert,omitempty"`

	// QPS is the API server request rate limit. Zero leaves the client-go
	// default (5). Hub typically sets 50–100.
	QPS float32 `json:"qps"`

	// Burst is the burst budget above QPS. Zero leaves the client-go
	// default (10).
	Burst int `json:"burst"`

	// Timeout is the per-request timeout applied to the REST client. Zero
	// means no explicit timeout (rely on context deadline).
	Timeout time.Duration `json:"timeout_ns"`

	// UserAgent is sent as the User-Agent header. Empty defaults to
	// "aisphere-kubernetesx".
	UserAgent string `json:"user_agent"`

	// FieldManager is the default SSA field manager used by Apply when
	// ApplyOptions.FieldManager is empty. Defaults to "aisphere-hub".
	FieldManager string `json:"field_manager"`

	// InsecureSkipVerify disables TLS certificate verification. This is
	// dangerous and intended only for local dev clusters. Production must
	// leave it false.
	InsecureSkipVerify bool `json:"insecure_skip_verify"`

	// Logger is the component logger. If nil, kubernetesx uses
	// logx.DefaultLogger().
	Logger logx.Logger `json:"-" yaml:"-"`

	// Metrics is the optional metrics manager for kubernetesx operations.
	// A nil Manager is a valid no-op.
	Metrics metricsx.Manager `json:"-" yaml:"-"`

	// MetricsEnabled controls whether kubernetesx operations record metrics.
	MetricsEnabled bool `json:"metrics_enabled" yaml:"metrics_enabled"`
}

// Validate returns ErrConfigInvalid if required fields are missing or
// inconsistent. A Config is valid when at least one of Host, Kubeconfig, or
// the in-cluster path is available; the in-cluster path is checked lazily at
// construction time, so Validate only rejects the impossible case where no
// connection source can be selected.
func (c Config) Validate() error {
	if c.FieldManager == "" {
		// default is allowed; nothing to reject here
	}
	if c.Timeout < 0 {
		return ErrConfigInvalid
	}
	if c.QPS < 0 {
		return ErrConfigInvalid
	}
	if c.Burst < 0 {
		return ErrConfigInvalid
	}
	// Host and Kubeconfig may both be empty when running in-cluster; that
	// case is resolved by the factory. Nothing to reject upfront.
	return nil
}

// Normalized returns a copy of c with default FieldManager / UserAgent filled
// in. It does not mutate c.
func (c Config) Normalized() Config {
	out := c
	if out.FieldManager == "" {
		out.FieldManager = DefaultFieldManager
	}
	if out.UserAgent == "" {
		out.UserAgent = DefaultUserAgent
	}
	if out.Logger == nil {
		out.Logger = logx.DefaultLogger()
	}
	return out
}

// MergeCredential merges a Credential into the receiver, returning a new
// Config. The Credential's Kubeconfig, Context, Token, and CACert override the
// matching Config fields. This is the canonical way Hub builds a per-cluster
// Config from a stored Credential.
//
// MergeCredential does not validate the credential; callers should call
// cred.Validate() first.
func (c Config) MergeCredential(cred Credential) (Config, error) {
	if cred.Kind == CredentialKindInCluster {
		// In-cluster: ignore Host/Kubeconfig, let the factory resolve from
		// the pod service account.
		c.Host = ""
		c.Kubeconfig = nil
		c.Context = ""
		return c, nil
	}
	if cred.Kind == CredentialKindKubeconfig {
		if len(cred.Kubeconfig) == 0 {
			return Config{}, errors.New("kubernetesx: kubeconfig credential has empty kubeconfig")
		}
		c.Kubeconfig = cred.Kubeconfig
		if cred.Context != "" {
			c.Context = cred.Context
		}
		return c, nil
	}
	if cred.Kind == CredentialKindServiceAccount {
		if cred.Host == "" {
			return Config{}, errors.New("kubernetesx: service-account credential requires Host")
		}
		c.Host = cred.Host
		c.Kubeconfig = nil
		c.Context = ""
		// Token and CACert are carried on Config so the factory can build a
		// fully authenticated *rest.Config from Config alone (MergeCredential
		// returns Config, not cred; without this the factory would construct
		// an unauthenticated client). Like Kubeconfig, these are secret
		// material on Config and must not be logged.
		c.Token = cred.Token
		c.CACert = cred.CACert
		return c, nil
	}
	return Config{}, ErrCredentialInvalid
}

// DefaultFieldManager is the SSA field manager used when Config.FieldManager
// and ApplyOptions.FieldManager are both empty.
const DefaultFieldManager = "aisphere-hub"

// DefaultUserAgent is the User-Agent sent when Config.UserAgent is empty.
const DefaultUserAgent = "aisphere-kubernetesx"
