// Package casdoor adapts the official Casdoor Go SDK to Kernel authn contracts.
package casdoor

import (
	"net/http"
	"os"
	"strings"
	"time"

	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

const ProviderName = "casdoor"

// Config contains Casdoor application configuration used by the SDK.
//
// The top-level fields describe the login/OIDC application that exchanges
// authorization codes and verifies user tokens. Admin describes the optional
// management application used for provisioning users, organizations,
// applications and groups.
type Config struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`

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
}

// AdminConfig is the Casdoor credential set for management APIs. In production
// this should normally be a dedicated admin application with narrowly scoped
// permissions instead of the public login application.
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
	c.JWTCertificate = strings.TrimSpace(firstNonEmpty(c.JWTCertificate, c.Certificate))
	c.JWTCertificateFile = strings.TrimSpace(c.JWTCertificateFile)
	c.Admin.JWTCertificate = strings.TrimSpace(firstNonEmpty(c.Admin.JWTCertificate, c.Admin.Certificate))
	c.Admin.JWTCertificateFile = strings.TrimSpace(c.Admin.JWTCertificateFile)
	return c
}

// Client implements authn.Authenticator, authn.TokenService and
// authn.IdentityAdmin using Casdoor's official Go SDK.
type Client struct {
	cfg Config
}

func New(cfg Config) (*Client, error) {
	cfg = cfg.Normalized()
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
	if cfg.HTTPClient != nil {
		// Casdoor SDK currently exposes SetHttpClient as package-global state.
		// Keep this adapter thin and document the side effect at construction.
		casdoorsdk.SetHttpClient(cfg.HTTPClient)
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg}, nil
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
	if cfg.JWTCertificate == "" {
		return errInvalidConfig("casdoor jwt certificate is required")
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
