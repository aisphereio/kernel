package casdoor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"golang.org/x/oauth2"
)

var _ authn.Authenticator = (*Client)(nil)
var _ authn.TokenService = (*Client)(nil)

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
	token, err := c.loginSDK(req.OrgID, req.AppID).GetOAuthToken(req.Code, req.State)
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, wrapBackend("casdoor code exchange failed", err)
	}
	principal, err := c.principalFromAccessToken(ctx, token.AccessToken, req.OrgID, req.AppID)
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, err
	}
	return tokenSetFromOAuth(token), principal, nil
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
	return c.principalFromAccessToken(ctx, req.Token, req.OrgID, req.AppID)
}

func (c *Client) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	// Casdoor Go SDK exposes token listing/deletion and introspection, but it does
	// not currently expose an OAuth revocation helper. Keep this explicit instead
	// of silently deleting by guessed token name.
	return authn.ErrIdentityBackendFailed("casdoor sdk does not expose token revocation", nil)
}

func (c *Client) principalFromAccessToken(ctx context.Context, token string, orgID, appID string) (authn.Principal, error) {
	_ = ctx
	claims, err := c.loginSDK(orgID, appID).ParseJwtToken(token)
	if err != nil {
		return authn.Principal{}, authn.ErrInvalidCredential("invalid casdoor token")
	}
	principal := principalFromClaims(claims, token, orgID, appID)
	if !principal.IsAuthenticated() {
		return authn.Principal{}, authn.ErrUnauthenticated("casdoor token has no authenticated subject")
	}
	return principal, nil
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
