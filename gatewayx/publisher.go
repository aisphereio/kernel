package gatewayx

import (
	"context"
	"fmt"
)

// RoutePublicationMode selects the control-plane backend that receives generated
// Gateway manifests. Services should publish through ManifestPublisher instead
// of directly depending on etcd, Kubernetes, Envoy Gateway, or any concrete
// registry implementation.
type RoutePublicationMode string

const (
	RoutePublicationModeUnspecified RoutePublicationMode = ""
	RoutePublicationModeEtcd        RoutePublicationMode = "etcd"
	RoutePublicationModeGatewayAPI  RoutePublicationMode = "gateway_api"
)

// ManifestPublisher is the provider-neutral route publication boundary.
//
// Implementations may store generated manifests in the legacy RouteRegistry
// shape, render/apply Kubernetes Gateway API resources, or forward to another
// control plane. The important boundary is that serverx only publishes generated
// Gateway manifests; it does not know whether the target is etcd or Kubernetes.
type ManifestPublisher interface {
	PublishManifest(ctx context.Context, manifest Manifest) error
}

// RouteRegistryPublisher adapts the existing RouteRegistry contract to the new
// ManifestPublisher interface. This keeps the current etcd/memory route registry
// path working while allowing new publishers such as Gateway API.
type RouteRegistryPublisher struct {
	Registry RouteRegistry
}

func NewRouteRegistryPublisher(registry RouteRegistry) RouteRegistryPublisher {
	return RouteRegistryPublisher{Registry: registry}
}

func (p RouteRegistryPublisher) PublishManifest(ctx context.Context, manifest Manifest) error {
	_ = ctx
	if p.Registry == nil {
		return fmt.Errorf("gatewayx: route registry publisher has nil registry")
	}
	return p.Registry.RegisterManifest(manifest)
}

// PublishManifests publishes a batch of manifests through a ManifestPublisher.
// Empty manifests are ignored after normalization/filtering by callers.
func PublishManifests(ctx context.Context, publisher ManifestPublisher, manifests ...Manifest) error {
	if publisher == nil {
		return fmt.Errorf("gatewayx: manifest publisher is nil")
	}
	for _, manifest := range manifests {
		if manifest.Service == "" && len(manifest.Routes) == 0 {
			continue
		}
		if err := publisher.PublishManifest(ctx, manifest); err != nil {
			return err
		}
	}
	return nil
}
