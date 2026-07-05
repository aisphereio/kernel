package authn

import "context"

// CompositeAuthenticator tries Primary first and falls back to Secondary only
// when the primary credential is missing/invalid for migration-friendly hybrid
// modes. Avoid using this as a long-term production default; prefer one clear
// authn mode per service.
type CompositeAuthenticator struct {
	Primary   Authenticator
	Secondary Authenticator
}

func (a CompositeAuthenticator) Authenticate(ctx context.Context, cred Credential) (Principal, error) {
	if a.Primary != nil {
		p, err := a.Primary.Authenticate(ctx, cred)
		if err == nil {
			return p, nil
		}
		if a.Secondary == nil || cred.Scheme != CredentialGatewayTrusted {
			return Principal{}, err
		}
	}
	if a.Secondary == nil {
		return Principal{}, ErrIdentityBackendFailed("authenticator is not configured", nil)
	}
	return a.Secondary.Authenticate(ctx, cred)
}
