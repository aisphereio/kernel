// Package casdoor adapts the official Casdoor Go SDK to Kernel authn contracts.
package casdoor

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

const ProviderName = "casdoor"

// Config contains Casdoor application configuration used by the SDK.
//
// The top-level fields describe the login/OIDC application that exchanges
// authorization codes and verifies user tokens. Admin describes the optional M2M/service application used for provisioning
// users, organizations, applications and groups. Do not configure this with
// built-in/admin credentials for normal runtime use.
type Config struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Issuer is the trusted OIDC issuer expected in Casdoor JWTs. When empty,
	// Endpoint is used as a compatibility default. For application-specific
	// discovery this should match the issuer returned by that discovery document.
	Issuer string `json:"issuer" yaml:"issuer"`

	// DiscoveryURL and JWKSURL enable standard OIDC/JWKS verification. Prefer
	// these in Gateway and production deployments. JWTCertificate remains as a
	// static fallback for local/dev setups.
	DiscoveryURL string `json:"discovery_url" yaml:"discovery_url"`
	JWKSURL      string `json:"jwks_url" yaml:"jwks_url"`

	// Audience limits tokens to the expected Casdoor application/client IDs.
	// When empty, ClientID is used as the default audience.
	Audience []string `json:"audience" yaml:"audience"`

	// AllowedOwners optionally restricts accepted Casdoor organizations.
	AllowedOwners []string `json:"allowed_owners" yaml:"allowed_owners"`

	// AllowedAlgs limits JWT signing algorithms accepted from Casdoor.
	// Default: RS256, RS512, ES256, ES512.
	AllowedAlgs []string `json:"allowed_algs" yaml:"allowed_algs"`

	// JWKSCacheTTL controls local JWKS cache lifetime.
	JWKSCacheTTL time.Duration `json:"jwks_cache_ttl_ns" yaml:"jwks_cache_ttl_ns"`

	OrganizationName string `json:"organization_name" yaml:"organization_name"`
	ApplicationName  string `json:"application_name" yaml:"application_name"`
	ClientID         string `json:"client_id" yaml:"client_id"`
	ClientSecret     string `json:"client_secret" yaml:"client_secret"`

	// JWTCertificate is the PEM public certificate used by Casdoor SDK to verify
	// JWT access/id tokens signed by the Casdoor application cert.
	JWTCertificate     string `json:"jwt_certificate" yaml:"jwt_certificate"`
	JWTCertificateFile string `json:"jwt_certificate_file" yaml:"jwt_certificate_file"`

	// Certificate is kept as a backward-compatible alias for JWTCertificate.
	// Prefer jwt_certificate or jwt_certificate_file in new configs.
	Certificate string `json:"certificate" yaml:"certificate"`

	Admin AdminConfig `json:"admin" yaml:"admin"`

	HTTPClient *http.Client  `json:"-" yaml:"-"`
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`

	// Logger is the component logger. If nil, casdoor uses logx.DefaultLogger().
	Logger logx.Logger `json:"-" yaml:"-"`

	// Metrics is the optional metrics manager for Casdoor backend HTTP calls.
	Metrics metricsx.Manager `json:"-" yaml:"-"`

	// MetricsEnabled controls whether Casdoor backend calls record metrics.
	MetricsEnabled bool `json:"metrics_enabled" yaml:"metrics_enabled"`
}

// AdminConfig is the Casdoor credential set for management APIs. In production
// this must be a dedicated service application using OAuth2 client_credentials
// / M2M semantics, for example `iam-service`. It should not use the public
// browser login application and should never use built-in/admin except for
// one-time bootstrap or break-glass recovery.
type AdminConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	OrganizationName string `json:"organization_name" yaml:"organization_name"`
	ApplicationName  string `json:"application_name" yaml:"application_name"`
	ClientID         string `json:"client_id" yaml:"client_id"`
	ClientSecret     string `json:"client_secret" yaml:"client_secret"`

	JWTCertificate     string `json:"jwt_certificate" yaml:"jwt_certificate"`
	JWTCertificateFile string `json:"jwt_certificate_file" yaml:"jwt_certificate_file"`

	// Certificate is kept as a backward-compatible alias for JWTCertificate.
	Certificate string `json:"certificate" yaml:"certificate"`
}

func (c Config) Normalized() Config {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.JWKSCacheTTL <= 0 {
		c.JWKSCacheTTL = 10 * time.Minute
	}
	c.Issuer = strings.TrimRight(strings.TrimSpace(c.Issuer), "/")
	c.DiscoveryURL = strings.TrimSpace(c.DiscoveryURL)
	c.JWKSURL = strings.TrimSpace(c.JWKSURL)
	c.Audience = normalizeStrings(c.Audience)
	c.AllowedOwners = normalizeStrings(c.AllowedOwners)
	c.AllowedAlgs = normalizeStrings(c.AllowedAlgs)
	if len(c.AllowedAlgs) == 0 {
		c.AllowedAlgs = []string{"RS256", "RS512", "ES256", "ES512"}
	}
	c.JWTCertificate = strings.TrimSpace(firstNonEmpty(c.JWTCertificate, c.Certificate))
	c.JWTCertificateFile = strings.TrimSpace(c.JWTCertificateFile)
	c.Admin.JWTCertificate = strings.TrimSpace(firstNonEmpty(c.Admin.JWTCertificate, c.Admin.Certificate))
	c.Admin.JWTCertificateFile = strings.TrimSpace(c.Admin.JWTCertificateFile)
	return c
}

// Client implements authn.Authenticator, authn.TokenService and
// authn.IdentityAdmin using Casdoor's official Go SDK.
type Client struct {
	cfg     Config
	logger  logx.Logger
	metrics metricsx.Manager
	jwks    *jwksKeySet
}

func New(cfg Config) (*Client, error) {
	cfg = cfg.Normalized()
	logger := casdoorLogger(cfg)
	registerCasdoorMetrics(cfg)
	started := time.Now()
	logger.Info("authn casdoor opening", logx.String("endpoint", cfg.Endpoint), logx.Bool("metrics_enabled", cfg.MetricsEnabled))
	var err error
	cfg.JWTCertificate, err = loadCertificate(cfg.JWTCertificate, cfg.JWTCertificateFile)
	if err != nil {
		return nil, errInvalidConfig("failed to load casdoor jwt certificate: " + err.Error())
	}
	cfg.Certificate = cfg.JWTCertificate
	if cfg.Admin.JWTCertificate != "" || cfg.Admin.JWTCertificateFile != "" || cfg.Admin.Certificate != "" {
		cfg.Admin.JWTCertificate, err = loadCertificate(cfg.Admin.JWTCertificate, cfg.Admin.JWTCertificateFile)
		if err != nil {
			return nil, errInvalidConfig("failed to load casdoor admin jwt certificate: " + err.Error())
		}
		cfg.Admin.Certificate = cfg.Admin.JWTCertificate
	}
	// Casdoor SDK currently exposes SetHttpClient as package-global state.
	// Install an instrumented HTTP client before validation-dependent SDK calls so
	// all later SDK traffic shares Kernel logx/errorx/metrics behavior.
	cfg.HTTPClient = instrumentHTTPClient(cfg.HTTPClient, cfg.Timeout, logger, cfg.Metrics, cfg.MetricsEnabled)
	casdoorsdk.SetHttpClient(cfg.HTTPClient)
	if err := validateConfig(cfg); err != nil {
		logger.Error("authn casdoor config invalid", logx.Duration("elapsed", time.Since(started)), logx.Err(err))
		return nil, err
	}
	logger.Info("authn casdoor opened", logx.Duration("elapsed", time.Since(started)))
	client := &Client{cfg: cfg, logger: logger, metrics: cfg.Metrics}
	if cfg.JWKSURL != "" || cfg.DiscoveryURL != "" {
		client.jwks = newJWKSKeySet(jwksConfig{
			DiscoveryURL: cfg.DiscoveryURL,
			JWKSURL:      cfg.JWKSURL,
			Issuer:       cfg.Issuer,
			CacheTTL:     cfg.JWKSCacheTTL,
			HTTPClient:   cfg.HTTPClient,
			Logger:       logger,
		})
	}
	return client, nil
}

func validateConfig(cfg Config) error {
	if cfg.Endpoint == "" {
		return errInvalidConfig("casdoor endpoint is required")
	}
	if cfg.ClientID == "" {
		return errInvalidConfig("casdoor client id is required")
	}
	if cfg.ClientSecret == "" {
		return errInvalidConfig("casdoor client secret is required")
	}
	if cfg.JWTCertificate == "" && cfg.JWKSURL == "" && cfg.DiscoveryURL == "" {
		return errInvalidConfig("casdoor jwt certificate or discovery_url/jwks_url is required")
	}
	if cfg.OrganizationName == "" {
		return errInvalidConfig("casdoor organization name is required")
	}
	if cfg.ApplicationName == "" {
		return errInvalidConfig("casdoor application name is required")
	}
	return nil
}

func loadCertificate(inline, file string) (string, error) {
	inline = strings.TrimSpace(inline)
	file = strings.TrimSpace(file)
	if inline != "" {
		return inline, nil
	}
	if file == "" {
		return "", nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (c *Client) loginSDK(orgID, appID string) *casdoorsdk.Client {
	org := firstNonEmpty(orgID, c.cfg.OrganizationName)
	app := firstNonEmpty(appID, c.cfg.ApplicationName)
	return casdoorsdk.NewClient(c.cfg.Endpoint, c.cfg.ClientID, c.cfg.ClientSecret, c.cfg.JWTCertificate, org, app)
}

func (c *Client) adminSDK() *casdoorsdk.Client {
	admin := c.cfg.Admin
	org := firstNonEmpty(admin.OrganizationName, c.cfg.OrganizationName)
	app := firstNonEmpty(admin.ApplicationName, c.cfg.ApplicationName)
	clientID := firstNonEmpty(admin.ClientID, c.cfg.ClientID)
	clientSecret := firstNonEmpty(admin.ClientSecret, c.cfg.ClientSecret)
	certificate := firstNonEmpty(admin.JWTCertificate, c.cfg.JWTCertificate)
	return casdoorsdk.NewClient(c.cfg.Endpoint, clientID, clientSecret, certificate, org, app)
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
