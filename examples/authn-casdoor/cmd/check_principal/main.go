// Command check_principal tests Casdoor token principal extraction.
//
// It uses the Kernel authn/casdoor adapter to:
// 1. Log in to Casdoor with admin/password via OAuth2 password grant
// 2. Parse the returned JWT using JWKS (auto-discovered from Casdoor)
// 3. Print the full Principal including SubjectID, Username, OrgID, etc.
//
// Usage:
//
//	cd examples/authn-casdoor
//	go run ./cmd/check_principal -config config.yaml -password "123456"
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	casdoorauthn "github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

type rootConfig struct {
	Log      logx.Config         `json:"log" yaml:"log"`
	Casdoor  legacyCasdoorConfig `json:"casdoor" yaml:"casdoor"`
	Security struct {
		Authn struct {
			Casdoor casdoorauthn.Config `json:"casdoor" yaml:"casdoor"`
		} `json:"authn" yaml:"authn"`
	} `json:"security" yaml:"security"`
	Example exampleConfig `json:"authn_example" yaml:"authn_example"`
}

type legacyCasdoorConfig struct {
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
	Organization string `json:"organization" yaml:"organization"`
	Application  string `json:"application" yaml:"application"`
	Certificate  string `json:"certificate" yaml:"certificate"`
	HTTPTimeout  string `json:"http_timeout" yaml:"http_timeout"`
}

type exampleConfig struct {
	Password string `json:"password" yaml:"password"`
}

func main() {
	configPath := flag.String("config", "examples/authn-casdoor/config.local.yaml", "YAML config path")
	username := flag.String("username", "admin", "Casdoor username to login")
	password := flag.String("password", "", "Casdoor password (or use config)")
	flag.Parse()

	ctx := context.Background()

	// Load config
	cfg, err := loadRootConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: load config: %v\n", err)
		os.Exit(1)
	}

	logger, closeLogger := newLogger(cfg.Log)
	defer closeLogger()

	// Resolve Casdoor config
	casdoorCfg, err := resolveCasdoorConfig(cfg)
	if err != nil {
		logger.Error("resolve casdoor config failed", logx.Err(err))
		os.Exit(1)
	}

	// Use password from flag or config
	pass := *password
	if pass == "" {
		pass = cfg.Example.Password
	}
	if pass == "" {
		logger.Error("password is required: use -password flag or set authn_example.password in config")
		os.Exit(1)
	}

	// Resolve org and app
	orgID := casdoorCfg.OrganizationName
	appID := casdoorCfg.ApplicationName
	clientID := casdoorCfg.ClientID
	clientSecret := casdoorCfg.ClientSecret
	endpoint := strings.TrimRight(strings.TrimSpace(casdoorCfg.Endpoint), "/")

	logger.Info("checking casdoor principal",
		logx.String("endpoint", endpoint),
		logx.String("org", orgID),
		logx.String("app", appID),
		logx.String("client_id", clientID),
		logx.String("username", *username),
	)

	// Step 1: Get access token via OAuth2 password grant
	tokenURL := endpoint + "/api/login/oauth/access_token"
	logger.Info("requesting token via password grant", logx.String("token_url", tokenURL))

	form := url.Values{
		"grant_type":    {"password"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"username":      {*username},
		"password":      {pass},
		"scope":         {"openid profile email"},
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.PostForm(tokenURL, form)
	if err != nil {
		logger.Error("token request failed", logx.Err(err))
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("read token response failed", logx.Err(err))
		os.Exit(1)
	}

	if resp.StatusCode != 200 {
		logger.Error("token request returned error",
			logx.Int("status", resp.StatusCode),
			logx.String("body", string(body)),
		)
		os.Exit(1)
	}

	// Parse OAuth2 token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		logger.Error("parse token response failed", logx.Err(err), logx.String("body", string(body)))
		os.Exit(1)
	}

	if tokenResp.AccessToken == "" {
		logger.Error("no access_token in response", logx.String("body", string(body)))
		os.Exit(1)
	}

	logger.Info("access token obtained",
		logx.String("token_hint", tokenHint(tokenResp.AccessToken)),
		logx.String("token_type", tokenResp.TokenType),
		logx.Int("expires_in", tokenResp.ExpiresIn),
		logx.String("scope", tokenResp.Scope),
		logx.Bool("has_refresh_token", tokenResp.RefreshToken != ""),
		logx.Bool("has_id_token", tokenResp.IDToken != ""),
	)

	// Step 2: Decode JWT without verification to see raw claims
	logger.Info("decoding raw JWT claims (no signature verification)")
	rawClaims := decodeJWTClaims(tokenResp.AccessToken)
	logRawClaims(logger, rawClaims)

	// Step 3: Use kernel authn framework with JWKS to verify and get Principal
	logger.Info("creating kernel casdoor authn adapter with JWKS discovery")

	// Configure JWKS for auto-discovery
	casdoorCfg.DiscoveryURL = endpoint + "/.well-known/openid-configuration"
	casdoorCfg.JWKSURL = "" // will be auto-discovered
	casdoorCfg.Issuer = "https://casdoor.weagent.cc:30723" // must match JWT iss claim
	casdoorCfg.AllowedOwners = []string{orgID}
	casdoorCfg.Audience = []string{clientID}

	client, err := casdoorauthn.New(casdoorCfg)
	if err != nil {
		logger.Error("create casdoor authn adapter failed", logx.Err(err))
		os.Exit(1)
	}

	// Verify the token through kernel framework
	logger.Info("verifying token through kernel authn framework")
	principal, err := client.VerifyToken(ctx, authn.VerifyTokenRequest{
		Token:     tokenResp.AccessToken,
		TokenType: "access_token",
		OrgID:     orgID,
		AppID:     appID,
	})
	if err != nil {
		logger.Error("token verification failed", logx.Err(err))
		os.Exit(1)
	}

	// Print full principal
	logPrincipal(logger, "principal_from_kernel", principal)

	// Also try Authenticate
	principal2, err := client.Authenticate(ctx, authn.Credential{
		Scheme: authn.CredentialBearer,
		Token:  tokenResp.AccessToken,
	})
	if err != nil {
		logger.Error("authenticate failed", logx.Err(err))
		os.Exit(1)
	}
	logPrincipal(logger, "principal_from_authenticate", principal2)

	// Print summary
	fmt.Println("\n========== PRINCIPAL SUMMARY ==========")
	fmt.Printf("  SubjectID:   %q\n", principal.SubjectID)
	fmt.Printf("  SubjectType: %q\n", principal.SubjectType)
	fmt.Printf("  Username:    %q\n", principal.Username)
	fmt.Printf("  Name:        %q\n", principal.Name)
	fmt.Printf("  Email:       %q\n", principal.Email)
	fmt.Printf("  Phone:       %q\n", principal.Phone)
	fmt.Printf("  OrgID:       %q\n", principal.OrgID)
	fmt.Printf("  AppID:       %q\n", principal.AppID)
	fmt.Printf("  ExternalID:  %q\n", principal.ExternalID)
	fmt.Printf("  Issuer:      %q\n", principal.Issuer)
	fmt.Printf("  Audience:    %v\n", principal.Audience)
	fmt.Printf("  Roles:       %v\n", principal.Roles)
	fmt.Printf("  Groups:      %v\n", principal.Groups)
	fmt.Printf("  AuthMethod:  %q\n", principal.AuthMethod)
	fmt.Printf("  Attributes:  %v\n", principal.Attributes)
	fmt.Printf("  IssuedAt:    %v\n", principal.IssuedAt)
	fmt.Printf("  ExpiresAt:   %v\n", principal.ExpiresAt)
	fmt.Println("========================================")
}

func loadRootConfig(path string) (rootConfig, error) {
	if path == "" {
		return rootConfig{}, errorx.BadRequest(errorx.CodeBadRequest, "config path is required")
	}
	cfg := configx.New(configx.WithSource(file.NewSource(path)))
	if err := cfg.Load(); err != nil {
		return rootConfig{}, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("load config failed"))
	}
	defer cfg.Close()
	var root rootConfig
	if err := cfg.Scan(&root); err != nil {
		return rootConfig{}, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("scan config failed"))
	}
	return root, nil
}

func resolveCasdoorConfig(root rootConfig) (casdoorauthn.Config, error) {
	cfg := root.Security.Authn.Casdoor
	if hasLegacyCasdoorConfig(root.Casdoor) {
		cfg.Endpoint = firstNonEmpty(root.Casdoor.Endpoint, cfg.Endpoint)
		cfg.OrganizationName = firstNonEmpty(root.Casdoor.Organization, cfg.OrganizationName)
		cfg.ApplicationName = firstNonEmpty(root.Casdoor.Application, cfg.ApplicationName)
		cfg.ClientID = firstNonEmpty(root.Casdoor.ClientID, cfg.ClientID)
		cfg.ClientSecret = firstNonEmpty(root.Casdoor.ClientSecret, cfg.ClientSecret)
		cfg.JWTCertificate = firstNonEmpty(root.Casdoor.Certificate, cfg.JWTCertificate, cfg.Certificate)
		cfg.JWTCertificateFile = firstNonEmpty(cfg.JWTCertificateFile)
		cfg.Certificate = firstNonEmpty(root.Casdoor.Certificate, cfg.Certificate)
		if root.Casdoor.HTTPTimeout != "" {
			d, err := time.ParseDuration(root.Casdoor.HTTPTimeout)
			if err != nil {
				return cfg, errorx.Wrap(err, errorx.CodeBadRequest, errorx.WithMessage("invalid casdoor.http_timeout"))
			}
			cfg.Timeout = d
		}
	}
	return cfg, nil
}

func hasLegacyCasdoorConfig(cfg legacyCasdoorConfig) bool {
	return firstNonEmpty(cfg.Endpoint, cfg.ClientID, cfg.ClientSecret, cfg.Organization, cfg.Application, cfg.Certificate) != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func newLogger(cfg logx.Config) (logx.Logger, func()) {
	if cfg.ServiceName == "" {
		cfg = logx.DefaultConfig("dev")
		cfg.ServiceName = "check-principal"
		cfg.Level = logx.DebugLevel.String()
		cfg.Format = logx.FormatConsole
		cfg.Output = string(logx.OutputStdout)
	}
	logger, _, err := logx.New(cfg)
	if err != nil {
		fallback := logx.Noop()
		fmt.Fprintf(os.Stderr, "failed to initialize logx: %v\n", err)
		return fallback, func() {}
	}
	return logger, func() { _ = logger.Sync() }
}

// decodeJWTClaims decodes a JWT payload without verifying the signature.
func decodeJWTClaims(token string) map[string]interface{} {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return map[string]interface{}{"error": "not a valid JWT"}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("decode payload failed: %v", err)}
		}
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("unmarshal claims failed: %v", err)}
	}
	return claims
}

func logRawClaims(logger logx.Logger, claims map[string]interface{}) {
	fields := make([]logx.Field, 0, len(claims))
	for k, v := range claims {
		switch val := v.(type) {
		case string:
			fields = append(fields, logx.String(k, val))
		case float64:
			fields = append(fields, logx.Float64(k, val))
		case bool:
			fields = append(fields, logx.Bool(k, val))
		default:
			b, _ := json.Marshal(val)
			fields = append(fields, logx.String(k, string(b)))
		}
	}
	logger.Info("raw_jwt_claims", fields...)
}

func tokenHint(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return token[:2] + "***"
	}
	return token[:6] + "..." + token[len(token)-6:]
}

func logPrincipal(logger logx.Logger, event string, p authn.Principal) {
	logger.Info(event,
		logx.String("subject_id", p.SubjectID),
		logx.String("subject_type", p.SubjectType),
		logx.String("provider", p.Provider),
		logx.String("external_id", p.ExternalID),
		logx.String("org", p.OrgID),
		logx.String("app", p.AppID),
		logx.String("username", p.Username),
		logx.String("name", p.Name),
		logx.String("email", p.Email),
		logx.Int("roles_count", len(p.Roles)),
		logx.Int("groups_count", len(p.Groups)),
		logx.Int("scopes_count", len(p.Scopes)),
		logx.Time("issued_at", p.IssuedAt),
		logx.Time("expires_at", p.ExpiresAt),
	)
}