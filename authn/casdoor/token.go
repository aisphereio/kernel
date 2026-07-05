package casdoor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/logx"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"
)

var _ authn.Authenticator = (*Client)(nil)
var _ authn.TokenService = (*Client)(nil)

const defaultTokenLeeway = 2 * time.Minute

func (c *Client) Authenticate(ctx context.Context, credential authn.Credential) (authn.Principal, error) {
	switch strings.ToLower(strings.TrimSpace(credential.Scheme)) {
	case "", authn.CredentialBearer:
		return c.VerifyToken(ctx, authn.VerifyTokenRequest{Token: credential.Token, TokenType: "access_token", Metadata: credential.Metadata})
	case authn.CredentialCode:
		_, principal, err := c.ExchangeCode(ctx, authn.AuthCodeExchangeRequest{Code: credential.Token, Metadata: credential.Metadata})
		return principal, err
	default:
		return authn.Principal{}, authn.ErrInvalidCredential("unsupported credential scheme")
	}
}

func (c *Client) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	if strings.TrimSpace(req.Code) == "" {
		return authn.TokenSet{}, authn.Principal{}, authn.ErrInvalidTokenRequest("authorization code is required")
	}
	token, err := c.exchangeOAuthToken(ctx, req)
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, wrapBackend("casdoor code exchange failed", err)
	}
	principal, err := c.principalFromAccessToken(ctx, token.AccessToken, req.OrgID, req.AppID, defaultTokenLeeway)
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, err
	}
	return tokenSetFromOAuth(token), principal, nil
}

func (c *Client) exchangeOAuthToken(ctx context.Context, req authn.AuthCodeExchangeRequest) (*oauth2.Token, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimRight(strings.TrimSpace(c.cfg.Endpoint), "/")
	config := oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		RedirectURL:  strings.TrimSpace(req.RedirectURI),
		Endpoint: oauth2.Endpoint{
			AuthURL:   endpoint + "/api/login/oauth/authorize",
			TokenURL:  endpoint + "/api/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	if c.cfg.HTTPClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.cfg.HTTPClient)
	}
	opts := make([]oauth2.AuthCodeOption, 0, 1)
	if verifier := strings.TrimSpace(req.CodeVerifier); verifier != "" {
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", verifier))
	}
	return config.Exchange(ctx, req.Code, opts...)
}

func (c *Client) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (authn.TokenSet, error) {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return authn.TokenSet{}, authn.ErrInvalidTokenRequest("refresh token is required")
	}
	token, err := c.loginSDK(req.OrgID, req.AppID).RefreshOAuthToken(req.RefreshToken)
	if err != nil {
		return authn.TokenSet{}, wrapBackend("casdoor token refresh failed", err)
	}
	return tokenSetFromOAuth(token), nil
}

func (c *Client) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	if strings.TrimSpace(req.Token) == "" {
		return authn.Principal{}, authn.ErrMissingCredential("token is required")
	}
	return c.principalFromAccessToken(ctx, req.Token, req.OrgID, req.AppID, tokenLeeway(req.Leeway))
}

func (c *Client) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	// Casdoor Go SDK exposes token listing/deletion and introspection, but it does
	// not currently expose an OAuth revocation helper. Keep this explicit instead
	// of silently deleting by guessed token name.
	return authn.ErrIdentityBackendFailed("casdoor sdk does not expose token revocation", nil)
}

func (c *Client) principalFromAccessToken(ctx context.Context, token string, orgID, appID string, leeway time.Duration) (authn.Principal, error) {
	_ = ctx
	claims, err := c.parseJWTToken(token, orgID, appID, leeway)
	if err != nil {
		logger := c.logger
		if logger == nil {
			logger = casdoorLogger(c.cfg)
		}
		logger.Warn("casdoor jwt parse failed", logx.Err(err), logx.Duration("leeway", leeway))
		return authn.Principal{}, authn.ErrInvalidCredential("invalid casdoor token")
	}
	principal := principalFromClaims(claims, token, orgID, appID)
	if !principal.IsAuthenticated() {
		return authn.Principal{}, authn.ErrUnauthenticated("casdoor token has no authenticated subject")
	}
	return principal, nil
}

func tokenLeeway(leeway time.Duration) time.Duration {
	if leeway > 0 {
		return leeway
	}
	return defaultTokenLeeway
}

func (c *Client) parseJWTToken(token string, orgID, appID string, leeway time.Duration) (*casdoorsdk.Claims, error) {
	claims := &claimsWithLeeway{leeway: tokenLeeway(leeway)}
	t, err := jwt.ParseWithClaims(token, claims, c.jwtKeyFunc(orgID, appID))
	if err != nil {
		return nil, err
	}
	if t == nil || !t.Valid {
		return nil, fmt.Errorf("token is invalid")
	}
	parsed, ok := t.Claims.(*claimsWithLeeway)
	if !ok {
		return nil, fmt.Errorf("unexpected casdoor claims type")
	}
	if err := c.validateClaims(&parsed.Claims, orgID, appID); err != nil {
		return nil, err
	}
	return &parsed.Claims, nil
}

func (c *Client) jwtKeyFunc(orgID, appID string) jwt.Keyfunc {
	client := c.loginSDK(orgID, appID)
	return func(token *jwt.Token) (any, error) {
		alg := token.Method.Alg()
		allowedAlgs := c.cfg.AllowedAlgs
		if len(allowedAlgs) == 0 {
			allowedAlgs = []string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodRS512.Alg(), jwt.SigningMethodES256.Alg(), jwt.SigningMethodES512.Alg()}
		}
		if !containsString(allowedAlgs, alg) {
			return nil, fmt.Errorf("unsupported signing method: %v", token.Header["alg"])
		}
		if c.jwks != nil {
			kid, _ := token.Header["kid"].(string)
			return c.jwks.key(context.Background(), kid)
		}
		switch alg {
		case jwt.SigningMethodES256.Alg():
			return jwt.ParseECPublicKeyFromPEM([]byte(client.Certificate))
		case jwt.SigningMethodES512.Alg():
			return jwt.ParseECPublicKeyFromPEM([]byte(client.Certificate))
		case jwt.SigningMethodRS256.Alg():
			return jwt.ParseRSAPublicKeyFromPEM([]byte(client.Certificate))
		case jwt.SigningMethodRS512.Alg():
			return jwt.ParseRSAPublicKeyFromPEM([]byte(client.Certificate))
		default:
			return nil, fmt.Errorf("unsupported signing method: %v", token.Header["alg"])
		}
	}
}

func (c *Client) validateClaims(claims *casdoorsdk.Claims, orgID, appID string) error {
	if claims == nil {
		return fmt.Errorf("empty claims")
	}
	expectedIssuer := strings.TrimRight(strings.TrimSpace(firstNonEmpty(c.cfg.Issuer, c.cfg.Endpoint)), "/")
	if expectedIssuer != "" && strings.TrimRight(strings.TrimSpace(claims.Issuer), "/") != expectedIssuer {
		return fmt.Errorf("invalid issuer %q", claims.Issuer)
	}
	audiences := c.cfg.Audience
	if len(audiences) == 0 {
		audiences = []string{firstNonEmpty(appID, c.cfg.ClientID)}
	}
	if len(audiences) > 0 {
		matched := false
		for _, aud := range audiences {
			if aud != "" && claims.VerifyAudience(aud, false) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("invalid audience %v", []string(claims.Audience))
		}
	}
	allowedOwners := c.cfg.AllowedOwners
	if len(allowedOwners) == 0 && orgID != "" {
		allowedOwners = []string{orgID}
	}
	if len(allowedOwners) > 0 && !containsString(allowedOwners, claims.Owner) {
		return fmt.Errorf("invalid casdoor owner %q", claims.Owner)
	}
	return nil
}

func containsString(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, v := range values {
		if strings.TrimSpace(v) == want {
			return true
		}
	}
	return false
}

type claimsWithLeeway struct {
	casdoorsdk.Claims
	leeway time.Duration
}

func (c *claimsWithLeeway) Valid() error {
	if c == nil {
		return nil
	}
	vErr := new(jwt.ValidationError)
	now := jwt.TimeFunc()
	leeway := c.leeway
	if leeway < 0 {
		leeway = 0
	}
	if !c.VerifyExpiresAt(now.Add(-leeway), false) {
		vErr.Inner = fmt.Errorf("%s by %s", jwt.ErrTokenExpired, now.Sub(c.ExpiresAt.Time))
		vErr.Errors |= jwt.ValidationErrorExpired
	}
	if !c.VerifyIssuedAt(now.Add(leeway), false) {
		vErr.Inner = jwt.ErrTokenUsedBeforeIssued
		vErr.Errors |= jwt.ValidationErrorIssuedAt
	}
	if !c.VerifyNotBefore(now.Add(leeway), false) {
		vErr.Inner = jwt.ErrTokenNotValidYet
		vErr.Errors |= jwt.ValidationErrorNotValidYet
	}
	if vErr.Errors == 0 {
		return nil
	}
	return vErr
}

func principalFromClaims(claims *casdoorsdk.Claims, accessToken string, orgID, appID string) authn.Principal {
	if claims == nil {
		return authn.Anonymous()
	}
	subjectID := firstNonEmpty(claims.Id, claims.ExternalId, claims.Subject, claims.Name)
	org := firstNonEmpty(orgID, claims.Owner)
	issuedAt := time.Time{}
	expiresAt := time.Time{}
	if claims.IssuedAt != nil {
		issuedAt = claims.IssuedAt.Time
	}
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	attrs := authn.AttributeSet{
		"casdoor_owner":       claims.Owner,
		"casdoor_name":        claims.Name,
		"casdoor_id":          claims.Id,
		"casdoor_external_id": claims.ExternalId,
		"casdoor_token_type":  claims.TokenType,
		"signin_method":       claims.SigninMethod,
		"access_token":        accessToken,
	}

	return authn.Principal{
		SubjectID:   subjectID,
		SubjectType: authn.SubjectTypeUser,
		Provider:    ProviderName,
		ExternalID:  fmt.Sprintf("%s/%s", claims.Owner, claims.Name),
		Issuer:      claims.Issuer,
		Audience:    []string(claims.Audience),
		OrgID:       org,
		AppID:       appID,
		Username:    claims.Name,
		Name:        firstNonEmpty(claims.DisplayName, claims.Name),
		Email:       claims.Email,
		Phone:       claims.Phone,
		Roles:       roleNames(claims.Roles),
		Groups:      append([]string(nil), claims.Groups...),
		AuthMethod:  authn.AuthMethodOIDC,
		Attributes:  attrs,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
	}.Normalize()
}

func roleNames(roles []*casdoorsdk.Role) []string {
	if len(roles) == 0 {
		return nil
	}
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		if role != nil && role.Name != "" {
			out = append(out, role.Name)
		}
	}
	return out
}

func tokenSetFromOAuth(token *oauth2.Token) authn.TokenSet {
	if token == nil {
		return authn.TokenSet{}
	}
	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	raw := authn.AttributeSet{}
	if v := token.Extra("scope"); v != nil {
		raw["scope"] = v
	}
	if v := token.Extra("id_token"); v != nil {
		raw["id_token"] = v
	}
	return authn.TokenSet{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      stringFromExtra(token.Extra("id_token")),
		TokenType:    tokenType,
		Scope:        stringFromExtra(token.Extra("scope")),
		ExpiresAt:    token.Expiry,
		Raw:          raw,
	}
}

func stringFromExtra(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return ""
	}
}
