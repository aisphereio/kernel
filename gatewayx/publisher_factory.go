package gatewayx

import "fmt"

// PublisherConfig chooses the route publication backend. It is intentionally
// small so business services can keep their config stable while deployments pick
// either the legacy route registry or the Kubernetes Gateway API path.
type PublisherConfig struct {
	Mode              RoutePublicationMode
	Registry          RouteRegistry
	GatewayAPIObjectSink GatewayAPIObjectSink
	GatewayAPI        GatewayAPIOptions
}

func NewManifestPublisher(cfg PublisherConfig) (ManifestPublisher, error) {
	switch cfg.Mode {
	case RoutePublicationModeUnspecified, RoutePublicationModeEtcd:
		if cfg.Registry == nil {
			return nil, fmt.Errorf("gatewayx: route registry is required for etcd publication")
		}
		return NewRouteRegistryPublisher(cfg.Registry), nil
	case RoutePublicationModeGatewayAPI:
		if cfg.GatewayAPIObjectSink == nil {
			return nil, fmt.Errorf("gatewayx: gateway api object sink is required")
		}
		return NewGatewayAPIPublisher(cfg.GatewayAPIObjectSink, cfg.GatewayAPI), nil
	default:
		return nil, fmt.Errorf("gatewayx: unsupported route publication mode %q", cfg.Mode)
	}
}
