package oidcx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/golang-jwt/jwt/v4"
)

var _ authn.Authenticator = (*Verifier)(nil)
var _ interface {
	VerifyToken(context.Context, authn.VerifyTokenRequest) (authn.Principal, error)
} = (*Verifier)(nil)

// Verifier verifies OIDC JWTs using configured issuer/audience and JWKS.
type Verifier struct {
	cfg  Config
	keys *keySet
}

func New(cfg Config) (*Verifier, error) {
	cfg = cfg.Normalized()
	if cfg.Issuer == "" && cfg.DiscoveryURL == "" {
		return nil, authn.ErrIdentityBackendFailed("oidc issuer or discovery_url is required", nil)
	}
	if cfg.JWKSURL == "" && cfg.DiscoveryURL == "" {
		return nil, authn.ErrIdentityBackendFailed("oidc jwks_url or discovery_url is required", nil)
	}
	return &Verifier{cfg: cfg, keys: newKeySet(cfg)}, nil
}

func (v *Verifier) Authenticate(ctx context.Context, credential authn.Credential) (authn.Principal, error) {
	if strings.TrimSpace(credential.Token) == "" {
		return authn.Principal{}, authn.ErrMissingCredential("token is required")
	}
	return v.VerifyToken(ctx, authn.VerifyTokenRequest{Token: credential.Token, Metadata: credential.Metadata})
}

func (v *Verifier) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	if strings.TrimSpace(req.Token) == "" {
		return authn.Principal{}, authn.ErrMissingCredential("token is required")
	}
	claims := jwt.MapClaims{}
	parser := jwt.Parser{SkipClaimsValidation: true}
	token, err := parser.ParseWithClaims(req.Token, claims, v.keyFunc(ctx))
	if err != nil {
		return authn.Principal{}, authn.ErrInvalidCredential("invalid oidc token: " + err.Error())
	}
	if token == nil || !token.Valid {
		return authn.Principal{}, authn.ErrInvalidCredential("invalid oidc token")
	}
	if err := v.validateClaims(ctx, claims, req); err != nil {
		return authn.Principal{}, authn.ErrInvalidCredential(err.Error())
	}
	principal := v.principalFromClaims(claims)
	if !principal.IsAuthenticated() {
		return authn.Principal{}, authn.ErrUnauthenticated("oidc token has no authenticated subject")
	}
	return principal, nil
}

func (v *Verifier) keyFunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		alg := token.Method.Alg()
		if !contains(v.cfg.AllowedAlgs, alg) {
			return nil, fmt.Errorf("unsupported signing method %q", alg)
		}
		kid, _ := token.Header["kid"].(string)
		return v.keys.key(ctx, kid)
	}
}

func (v *Verifier) validateClaims(ctx context.Context, claims jwt.MapClaims, req authn.VerifyTokenRequest) error {
	expectedIssuer := strings.TrimRight(strings.TrimSpace(firstNonEmpty(req.Issuer, v.cfg.Issuer)), "/")
	if expectedIssuer == "" {
		issuer, err := v.keys.issuerValue(ctx)
		if err != nil {
			return err
		}
		expectedIssuer = issuer
	}
	if expectedIssuer != "" {
		actual := strings.TrimRight(strings.TrimSpace(stringClaim(claims, "iss")), "/")
		if actual != expectedIssuer {
			return fmt.Errorf("invalid issuer %q", actual)
		}
	}
	audiences := req.Audience
	if len(audiences) == 0 {
		audiences = v.cfg.Audience
	}
	if len(audiences) > 0 && !audienceMatches(claims, audiences) {
		return fmt.Errorf("invalid audience %v", claimValue(claims, "aud"))
	}
	now := time.Now()
	skew := v.cfg.ClockSkew
	if req.Leeway > 0 {
		skew = req.Leeway
	}
	if exp, ok := numericDate(claims, "exp"); ok && now.After(exp.Add(skew)) {
		return fmt.Errorf("token expired")
	}
	if nbf, ok := numericDate(claims, "nbf"); ok && now.Add(skew).Before(nbf) {
		return fmt.Errorf("token not valid yet")
	}
	if iat, ok := numericDate(claims, "iat"); ok && now.Add(skew).Before(iat) {
		return fmt.Errorf("token issued in the future")
	}
	if len(v.cfg.AllowedOwners) > 0 {
		owner := stringClaim(claims, v.cfg.OwnerClaim)
		if !contains(v.cfg.AllowedOwners, owner) {
			return fmt.Errorf("invalid owner %q", owner)
		}
	}
	return nil
}

func (v *Verifier) principalFromClaims(claims jwt.MapClaims) authn.Principal {
	subject := firstNonEmpty(stringClaim(claims, v.cfg.SubjectIDClaim), stringClaim(claims, "sub"), stringClaim(claims, v.cfg.UsernameClaim))
	username := firstNonEmpty(stringClaim(claims, v.cfg.UsernameClaim), stringClaim(claims, "preferred_username"), stringClaim(claims, "sub"))
	owner := stringClaim(claims, v.cfg.OwnerClaim)
	name := firstNonEmpty(stringClaim(claims, v.cfg.NameClaim), stringClaim(claims, "name"), username)
	issuedAt := time.Time{}
	if iat, ok := numericDate(claims, "iat"); ok {
		issuedAt = iat
	}
	expiresAt := time.Time{}
	if exp, ok := numericDate(claims, "exp"); ok {
		expiresAt = exp
	}
	attrs := authn.AttributeSet{}
	for k, val := range claims {
		attrs[k] = val
	}
	return authn.Principal{
		SubjectID:   subject,
		SubjectType: authn.SubjectTypeUser,
		Provider:    v.cfg.Provider,
		ExternalID:  externalID(owner, username, subject),
		Issuer:      strings.TrimRight(strings.TrimSpace(stringClaim(claims, "iss")), "/"),
		Audience:    stringSliceClaim(claims, "aud"),
		OrgID:       owner,
		TenantID:    owner,
		Username:    username,
		Name:        name,
		Email:       stringClaim(claims, v.cfg.EmailClaim),
		Groups:      stringSliceClaim(claims, v.cfg.GroupsClaim),
		Roles:       stringSliceClaim(claims, v.cfg.RolesClaim),
		Scopes:      scopeClaim(claims, v.cfg.ScopesClaim),
		AuthMethod:  authn.AuthMethodOIDC,
		Attributes:  attrs,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
	}.Normalize()
}

func externalID(owner, username, subject string) string {
	if owner != "" && username != "" {
		return owner + "/" + username
	}
	if username != "" {
		return username
	}
	return subject
}

func contains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, v := range values {
		if strings.TrimSpace(v) == want {
			return true
		}
	}
	return false
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func claimValue(claims jwt.MapClaims, key string) any {
	if claims == nil {
		return nil
	}
	return claims[key]
}
func stringClaim(claims jwt.MapClaims, key string) string {
	v := claimValue(claims, key)
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case fmt.Stringer:
		return strings.TrimSpace(x.String())
	default:
		return ""
	}
}
func stringSliceClaim(claims jwt.MapClaims, key string) []string {
	v := claimValue(claims, key)
	switch x := v.(type) {
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.Contains(x, " ") && !strings.Contains(x, ",") {
			return strings.Fields(x)
		}
		parts := strings.Split(x, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
func scopeClaim(claims jwt.MapClaims, key string) []string { return stringSliceClaim(claims, key) }
func audienceMatches(claims jwt.MapClaims, accepted []string) bool {
	auds := stringSliceClaim(claims, "aud")
	for _, aud := range auds {
		if contains(accepted, aud) {
			return true
		}
	}
	return false
}
func numericDate(claims jwt.MapClaims, key string) (time.Time, bool) {
	v := claimValue(claims, key)
	switch x := v.(type) {
	case float64:
		return time.Unix(int64(x), 0), true
	case int64:
		return time.Unix(x, 0), true
	case int:
		return time.Unix(int64(x), 0), true
	case jsonNumber:
		if n, err := x.Int64(); err == nil {
			return time.Unix(n, 0), true
		}
	}
	return time.Time{}, false
}

type jsonNumber interface{ Int64() (int64, error) }
