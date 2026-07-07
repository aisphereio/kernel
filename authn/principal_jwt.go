package authn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	DefaultPrincipalJWTHeader = "X-Aisphere-Principal-JWT"
	CredentialPrincipalJWT    = "principal_jwt"
	PrincipalJWTType          = "aisphere-principal"
)

type PrincipalJWTConfig struct {
	Enabled   bool          `json:"enabled" yaml:"enabled"`
	Header    string        `json:"header" yaml:"header"`
	Secret    string        `json:"secret" yaml:"secret"`
	Issuer    string        `json:"issuer" yaml:"issuer"`
	Audience  []string      `json:"audience" yaml:"audience"`
	TTL       time.Duration `json:"ttl_ns" yaml:"ttl_ns"`
	ClockSkew time.Duration `json:"clock_skew_ns" yaml:"clock_skew_ns"`
}

func (c PrincipalJWTConfig) Normalized() PrincipalJWTConfig {
	c.Header = strings.TrimSpace(c.Header)
	if c.Header == "" { c.Header = DefaultPrincipalJWTHeader }
	c.Secret = strings.TrimSpace(c.Secret)
	c.Issuer = strings.TrimSpace(c.Issuer)
	if c.Issuer == "" { c.Issuer = "aisphere-iam" }
	if len(c.Audience) == 0 { c.Audience = []string{"aisphere-internal"} }
	if c.TTL <= 0 { c.TTL = 5 * time.Minute }
	if c.ClockSkew <= 0 { c.ClockSkew = 30 * time.Second }
	return c
}

type PrincipalJWTClaims struct {
	jwt.RegisteredClaims
	Type string `json:"typ,omitempty"`
	SubjectType string `json:"subject_type,omitempty"`
	Provider string `json:"provider,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	OrgID string `json:"org_id,omitempty"`
	AppID string `json:"app_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Username string `json:"username,omitempty"`
	Name string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	Roles []string `json:"roles,omitempty"`
	Groups []string `json:"groups,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
	AuthMethod string `json:"auth_method,omitempty"`
}

func SignPrincipalJWT(p Principal, cfg PrincipalJWTConfig) (string, error) {
	cfg = cfg.Normalized()
	if cfg.Secret == "" { return "", ErrInvalidTokenRequest("principal_jwt secret is required") }
	p = p.Normalize()
	if !p.IsAuthenticated() { return "", ErrUnauthenticated("principal is not authenticated") }
	now := time.Now().UTC()
	claims := PrincipalJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{Issuer: cfg.Issuer, Subject: p.SubjectID, Audience: jwt.ClaimStrings(append([]string(nil), cfg.Audience...)), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-cfg.ClockSkew)), ExpiresAt: jwt.NewNumericDate(now.Add(cfg.TTL))},
		Type: PrincipalJWTType, SubjectType: p.SubjectType, Provider: p.Provider, ExternalID: p.ExternalID, TenantID: p.TenantID, OrgID: p.OrgID, AppID: p.AppID, ProjectID: p.ProjectID, Username: p.Username, Name: p.Name, Email: p.Email, Phone: p.Phone, Roles: append([]string(nil), p.Roles...), Groups: append([]string(nil), p.Groups...), Scopes: append([]string(nil), p.Scopes...), AuthMethod: p.AuthMethod,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.Secret))
}

type PrincipalJWTAuthenticator struct { Config PrincipalJWTConfig }

func (a PrincipalJWTAuthenticator) Authenticate(ctx context.Context, cred Credential) (Principal, error) {
	_ = ctx
	cfg := a.Config.Normalized()
	if cfg.Secret == "" { return Principal{}, ErrIdentityBackendFailed("principal_jwt secret is not configured", nil) }
	if strings.TrimSpace(cred.Token) == "" { return Principal{}, ErrMissingCredential("principal jwt is required") }
	claims := &PrincipalJWTClaims{}
	parser := jwt.Parser{ValidMethods: []string{jwt.SigningMethodHS256.Alg()}, SkipClaimsValidation: true}
	token, err := parser.ParseWithClaims(cred.Token, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok { return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"]) }
		return []byte(cfg.Secret), nil
	})
	if err != nil || token == nil || !token.Valid { return Principal{}, ErrInvalidCredential("invalid principal jwt") }
	if err := validatePrincipalJWTClaims(claims, cfg); err != nil { return Principal{}, err }
	return claims.Principal(), nil
}

func validatePrincipalJWTClaims(claims *PrincipalJWTClaims, cfg PrincipalJWTConfig) error {
	if claims == nil { return ErrInvalidCredential("invalid principal jwt claims") }
	if claims.Type != "" && claims.Type != PrincipalJWTType { return ErrInvalidCredential("invalid principal jwt type") }
	if strings.TrimSpace(claims.Subject) == "" { return ErrInvalidCredential("principal jwt subject is required") }
	if cfg.Issuer != "" && claims.Issuer != cfg.Issuer { return ErrInvalidCredential("invalid principal jwt issuer") }
	if len(cfg.Audience) > 0 && !audienceAllowed(claims.Audience, cfg.Audience) { return ErrInvalidCredential("invalid principal jwt audience") }
	now := time.Now().UTC()
	if claims.ExpiresAt == nil || now.After(claims.ExpiresAt.Time.Add(cfg.ClockSkew)) { return ErrInvalidCredential("principal jwt is expired") }
	if claims.NotBefore != nil && now.Add(cfg.ClockSkew).Before(claims.NotBefore.Time) { return ErrInvalidCredential("principal jwt is not valid yet") }
	return nil
}

func audienceAllowed(actual jwt.ClaimStrings, allowed []string) bool {
	for _, a := range actual { for _, b := range allowed { if strings.TrimSpace(a) != "" && strings.TrimSpace(a) == strings.TrimSpace(b) { return true } } }
	return false
}

func (c PrincipalJWTClaims) Principal() Principal {
	return Principal{SubjectID: c.Subject, SubjectType: c.SubjectType, Provider: c.Provider, ExternalID: c.ExternalID, Issuer: c.Issuer, Audience: append([]string(nil), c.Audience...), TenantID: c.TenantID, OrgID: c.OrgID, AppID: c.AppID, ProjectID: c.ProjectID, Username: c.Username, Name: c.Name, Email: c.Email, Phone: c.Phone, Roles: append([]string(nil), c.Roles...), Groups: append([]string(nil), c.Groups...), Scopes: append([]string(nil), c.Scopes...), AuthMethod: c.AuthMethod, IssuedAt: timeFromNumericDate(c.IssuedAt), ExpiresAt: timeFromNumericDate(c.ExpiresAt)}.Normalize()
}

func timeFromNumericDate(d *jwt.NumericDate) time.Time { if d == nil { return time.Time{} }; return d.Time }
