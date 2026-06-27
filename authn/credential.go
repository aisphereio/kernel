package authn

import (
	"context"
	"strings"
)

func BearerCredential(header string) (Credential, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return Credential{}, false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return Credential{}, false
	}
	return Credential{Scheme: CredentialBearer, Token: parts[1]}, true
}

// TokenAuthenticator adapts a TokenService/TokenVerifier to Authenticator.
type TokenAuthenticator struct {
	Verifier TokenVerifier
	Issuer   string
	Audience []string
	OrgID    string
	AppID    string
}

func (a TokenAuthenticator) Authenticate(ctx context.Context, credential Credential) (Principal, error) {
	if strings.TrimSpace(credential.Token) == "" {
		return Principal{}, ErrMissingCredential("")
	}
	if a.Verifier == nil {
		return Principal{}, ErrInvalidCredential("authn verifier is not configured")
	}
	principal, err := a.Verifier(ctx, VerifyTokenRequest{
		Token:    credential.Token,
		Issuer:   a.Issuer,
		Audience: a.Audience,
		OrgID:    a.OrgID,
		AppID:    a.AppID,
		Metadata: credential.Metadata,
	})
	if err != nil {
		return Principal{}, ErrInvalidCredential(err.Error())
	}
	principal = principal.Normalize()
	if !principal.IsAuthenticated() {
		return Principal{}, ErrInvalidCredential("verifier returned empty subject")
	}
	return principal, nil
}
