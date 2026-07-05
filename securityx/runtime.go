package securityx

import (
	"context"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/cachex"
)

// Runtime is the security material produced from Config and consumed by
// serverx.RuntimeProviders. It deliberately contains provider-neutral runtime
// objects, not transport middleware, so serverx/autowire remains the single
// middleware assembly layer.
type Runtime struct {
	Config Config

	AuthnBoundary      *AuthnBoundaryRuntime
	SkipPolicyResolver accessx.SkipPolicyResolver
}

// NewRuntime builds the security runtime from the canonical security config.
// cache is optional and is only used by JWT/OIDC authenticators when CacheTTL is
// configured.
func NewRuntime(ctx context.Context, cfg Config, cache cachex.Cache) (*Runtime, error) {
	cfg = cfg.Normalized()
	authnRuntime, err := NewAuthnBoundaryRuntime(ctx, cfg.Authn, cache)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		Config:             cfg,
		AuthnBoundary:      authnRuntime,
		SkipPolicyResolver: accessx.NewSkipPolicyResolver(cfg.Access),
	}, nil
}
