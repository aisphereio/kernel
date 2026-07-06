package serverx

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/gatewayx"
)

// PublishGatewayRoutes publishes the catalog's generated Gateway manifests
// through a provider-neutral ManifestPublisher. Use this when the route control
// plane may be etcd, Kubernetes Gateway API, GitOps, or another backend.
func (c ServiceCatalog) PublishGatewayRoutes(ctx context.Context, publisher gatewayx.ManifestPublisher) error {
	return c.PublishGatewayRoutesWithFilter(ctx, publisher, gatewayx.RouteFilter{})
}

// PublishGatewayRoutesWithFilter applies a deployment profile filter before
// publishing routes through the selected backend.
func (c ServiceCatalog) PublishGatewayRoutesWithFilter(ctx context.Context, publisher gatewayx.ManifestPublisher, filter gatewayx.RouteFilter) error {
	return PublishServiceGatewayRoutesWithFilter(ctx, publisher, filter, c.modules...)
}

// PublishServiceGatewayRoutes publishes generated service module manifests
// without filtering.
func PublishServiceGatewayRoutes(ctx context.Context, publisher gatewayx.ManifestPublisher, modules ...ServiceModule) error {
	return PublishServiceGatewayRoutesWithFilter(ctx, publisher, gatewayx.RouteFilter{}, modules...)
}

// PublishServiceGatewayRoutesWithFilter is the backend-neutral route publishing
// path. The legacy etcd RouteRegistry path is available through
// gatewayx.NewRouteRegistryPublisher(registry); Gateway API publication uses
// gatewayx.NewGatewayAPIPublisher(sink, opts).
func PublishServiceGatewayRoutesWithFilter(ctx context.Context, publisher gatewayx.ManifestPublisher, filter gatewayx.RouteFilter, modules ...ServiceModule) error {
	if publisher == nil {
		return fmt.Errorf("serverx: gateway manifest publisher is nil")
	}
	for _, module := range modules {
		manifest := module.GatewayManifest
		if manifest.Service == "" && len(manifest.Routes) == 0 {
			continue
		}
		manifest = gatewayx.FilterManifest(manifest, filter)
		if manifest.Service == "" && len(manifest.Routes) == 0 {
			continue
		}
		if len(manifest.Routes) == 0 {
			continue
		}
		if err := publisher.PublishManifest(ctx, manifest); err != nil {
			return err
		}
	}
	return nil
}
