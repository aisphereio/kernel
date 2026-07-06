package gatewayx

import (
	"context"
	"fmt"
)

const GatewayAPIVersion = "gateway.networking.k8s.io/v1"

type KubernetesObject struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   ObjectMeta     `json:"metadata"`
	Spec       map[string]any `json:"spec"`
}

type ObjectMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type GatewayAPIObjectSink interface {
	ApplyGatewayAPIObjects(ctx context.Context, objects []KubernetesObject) error
}

type GatewayAPIOptions struct {
	Namespace   string
	GatewayName string
	Hostnames   []string
}

type GatewayAPIPublisher struct {
	Sink    GatewayAPIObjectSink
	Options GatewayAPIOptions
}

func NewGatewayAPIPublisher(sink GatewayAPIObjectSink, opts GatewayAPIOptions) GatewayAPIPublisher {
	return GatewayAPIPublisher{Sink: sink, Options: opts}
}

func (p GatewayAPIPublisher) PublishManifest(ctx context.Context, manifest Manifest) error {
	if p.Sink == nil {
		return fmt.Errorf("gatewayx: gateway api publisher has nil object sink")
	}
	objects, err := RenderGatewayAPIObjects(manifest, p.Options)
	if err != nil {
		return err
	}
	return p.Sink.ApplyGatewayAPIObjects(ctx, objects)
}

type MemoryGatewayAPIObjectSink struct {
	Objects []KubernetesObject
}

func (s *MemoryGatewayAPIObjectSink) ApplyGatewayAPIObjects(ctx context.Context, objects []KubernetesObject) error {
	_ = ctx
	s.Objects = append(s.Objects, objects...)
	return nil
}

func RenderGatewayAPIObjects(manifest Manifest, opts GatewayAPIOptions) ([]KubernetesObject, error) {
	manifest = normalizeManifest(manifest)
	if manifest.Service == "" || len(manifest.Routes) == 0 {
		return nil, nil
	}
	if opts.Namespace == "" {
		opts.Namespace = manifest.Namespace
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.GatewayName == "" {
		opts.GatewayName = "aisphere-public"
	}
	if len(opts.Hostnames) == 0 {
		opts.Hostnames = []string{"*"}
	}
	rules := make([]map[string]any, 0, len(manifest.Routes))
	for _, route := range manifest.Routes {
		rules = append(rules, map[string]any{
			"name": route.ID,
			"matches": []map[string]any{{
				"method": route.Method,
				"path": map[string]any{"type": "PathPrefix", "value": route.Path},
			}},
			"backendRefs": []map[string]any{{"name": route.Upstream.Service, "port": route.Upstream.Port}},
		})
	}
	return []KubernetesObject{{
		APIVersion: GatewayAPIVersion,
		Kind:       "HTTPRoute",
		Metadata: ObjectMeta{
			Name:      manifest.Service + "-http",
			Namespace: opts.Namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "aisphere-kernel"},
		},
		Spec: map[string]any{
			"parentRefs": []map[string]any{{"name": opts.GatewayName}},
			"hostnames":  opts.Hostnames,
			"rules":      rules,
		},
	}}, nil
}
