package authn

import "context"

const CredentialGatewayTrusted = "gateway_trusted"

// TrustedHeaderAuthenticator trusts a Principal already verified and injected by
// Gateway. Use only behind network policy / mTLS / service-token boundaries that
// prevent clients from bypassing Gateway.
type TrustedHeaderAuthenticator struct {
	// InternalToken optionally requires a Gateway-to-backend shared secret before
	// trusting X-Aisphere-* identity headers. Use this in gateway_trusted mode.
	InternalToken InternalServiceTokenConfig
}

func (a TrustedHeaderAuthenticator) Authenticate(ctx context.Context, credential Credential) (Principal, error) {
	_ = ctx
	if a.InternalToken.Enabled {
		got := internalServiceTokenFromMetadata(credential.Metadata, a.InternalToken.Header())
		if !a.InternalToken.ValidateToken(got) {
			return Principal{}, ErrInvalidCredential("invalid gateway internal service token")
		}
	}
	p, ok := PrincipalFromTrustedHeaders(credential.Metadata)
	if !ok {
		return Principal{}, ErrMissingCredential("trusted gateway identity headers are required")
	}
	return p, nil
}
