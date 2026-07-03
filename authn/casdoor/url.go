package casdoor

import (
	"context"
	"net/url"
	"strings"

	"github.com/aisphereio/kernel/authn"
)

var _ authn.LoginService = (*Client)(nil)
var _ authn.LogoutService = (*Client)(nil)

// BuildLoginURL returns a Casdoor hosted login URL through Kernel's provider-
// neutral LoginService contract. Callers should not import or call the Casdoor
// SDK directly.
func (c *Client) BuildLoginURL(ctx context.Context, req authn.LoginURLRequest) (authn.LoginURL, error) {
	_ = ctx
	redirectURI := strings.TrimSpace(req.RedirectURI)
	if redirectURI == "" {
		return authn.LoginURL{}, authn.ErrInvalidTokenRequest("redirect uri is required")
	}
	orgID := firstNonEmpty(req.OrgID, c.cfg.OrganizationName)
	appID := firstNonEmpty(req.AppID, c.cfg.ApplicationName)
	rawURL := c.loginSDK(orgID, appID).GetSigninUrl(redirectURI)
	rawURL = withQuery(rawURL, map[string]string{
		"scope": strings.TrimSpace(req.Scope),
		"state": strings.TrimSpace(req.State),
	})
	return authn.LoginURL{
		URL:         rawURL,
		RedirectURI: redirectURI,
		State:       strings.TrimSpace(req.State),
		Scope:       strings.TrimSpace(req.Scope),
		Provider:    ProviderName,
		OrgID:       orgID,
		AppID:       appID,
	}, nil
}

// HandleCallback exchanges the authorization code and verifies the resulting
// token through Kernel authn contracts. It also validates state when an
// ExpectedState is provided.
func (c *Client) HandleCallback(ctx context.Context, req authn.CallbackRequest) (authn.CallbackResult, error) {
	if strings.TrimSpace(req.Code) == "" {
		return authn.CallbackResult{}, authn.ErrInvalidTokenRequest("authorization code is required")
	}
	if req.ExpectedState != "" && req.State != "" && req.State != req.ExpectedState {
		return authn.CallbackResult{}, authn.ErrInvalidCredential("oauth callback state mismatch")
	}
	tokens, principal, err := c.ExchangeCode(ctx, authn.AuthCodeExchangeRequest{
		Code:        req.Code,
		State:       req.State,
		RedirectURI: req.RedirectURI,
		OrgID:       req.OrgID,
		AppID:       req.AppID,
		Metadata:    req.Metadata,
	})
	if err != nil {
		return authn.CallbackResult{}, err
	}
	return authn.CallbackResult{
		Tokens:    tokens,
		Principal: principal,
		State:     req.State,
		Provider:  ProviderName,
	}, nil
}

// BuildLogoutURL returns a Casdoor hosted logout (end-session) URL through
// Kernel's provider-neutral LogoutService contract.
//
// The URL is built against the Casdoor /api/logout endpoint with OIDC
// RP-Initiated Logout parameters (client_id, post_logout_redirect_uri,
// id_token_hint, state). We construct the URL manually rather than calling
// the Casdoor SDK's GetSignoutUrl() because that SDK method returns the
// internal /login/oauth/logout path, while external OAuth applications
// should use /api/logout per the OIDC end-session endpoint convention.
func (c *Client) BuildLogoutURL(ctx context.Context, req authn.LogoutURLRequest) (authn.LogoutURL, error) {
	_ = ctx
	endpoint := strings.TrimRight(strings.TrimSpace(c.cfg.Endpoint), "/")
	if endpoint == "" {
		return authn.LogoutURL{}, authn.ErrInvalidTokenRequest("casdoor endpoint is not configured")
	}
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	if req.PostLogoutRedirectURI != "" {
		q.Set("post_logout_redirect_uri", req.PostLogoutRedirectURI)
	}
	if req.IDTokenHint != "" {
		q.Set("id_token_hint", req.IDTokenHint)
	}
	if req.State != "" {
		q.Set("state", req.State)
	}
	return authn.LogoutURL{
		URL:                   endpoint + "/api/logout?" + q.Encode(),
		PostLogoutRedirectURI: req.PostLogoutRedirectURI,
		State:                 req.State,
		Provider:              ProviderName,
		OrgID:                 req.OrgID,
		AppID:                 req.AppID,
	}, nil
}

// SigninURL is kept as a convenience alias for existing integrations. New code
// should prefer BuildLoginURL via the authn.LoginService interface.
func (c *Client) SigninURL(redirectURI string) string {
	out, err := c.BuildLoginURL(context.Background(), authn.LoginURLRequest{RedirectURI: redirectURI})
	if err != nil {
		return ""
	}
	return out.URL
}

// SignupURL returns the Casdoor hosted signup URL for the configured login
// application. Signup is provider-specific for now and intentionally not part
// of the core authn.LoginService contract.
func (c *Client) SignupURL(enablePassword bool, redirectURI string) string {
	return c.loginSDK("", "").GetSignupUrl(enablePassword, redirectURI)
}

func withQuery(rawURL string, values map[string]string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for k, v := range values {
		if strings.TrimSpace(v) != "" {
			q.Set(k, strings.TrimSpace(v))
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
