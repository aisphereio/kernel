package authn

import "context"

// LoginService is the provider-neutral browser login contract used by HTTP
// handlers, examples and application boot code. Implementations can use
// Casdoor/OIDC SDKs internally, but callers only depend on Kernel authn types.
type LoginService interface {
	BuildLoginURL(ctx context.Context, req LoginURLRequest) (LoginURL, error)
	HandleCallback(ctx context.Context, req CallbackRequest) (CallbackResult, error)
}
