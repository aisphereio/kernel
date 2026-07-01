package serverx

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/gatewayx"
)

// RegisterGatewayRoutes publishes generated route manifests through the Kernel
// route registry abstraction. In local validation this is normally a
// gatewayx.MemoryRegistry; production should provide an etcd-backed registry.
func RegisterGatewayRoutes(ctx context.Context, registry gatewayx.RouteRegistry, manifests ...gatewayx.Manifest) error {
	_ = ctx
	if registry == nil {
		return fmt.Errorf("serverx: gateway route registry is not configured")
	}
	for _, manifest := range manifests {
		if err := registry.RegisterManifest(manifest); err != nil {
			return err
		}
	}
	return nil
}
