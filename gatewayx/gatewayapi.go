package gatewayx

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
)

const (
	GatewayAPIVersion      = "gateway.networking.k8s.io/v1"
	AispherePolicyVersion = "security.aisphere.io/v1alpha1"
)

// KubernetesObject is a dependency-light representation of a Kubernetes object.
//
// Kernel intentionally does not import Kubernetes or Gateway API client packages
// here. Production code can adapt these objects to unstructured.Unstructured or
// marshal them to JSON/YAML for GitOps.
type KubernetesObject struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   ObjectMeta     `json:"metadata"`
	Spec       map[string]any `json:"spec"`
}

type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewayAPIObjectSink receives rendered Kubernetes Gateway API objects.
// Implementations may apply them to the Kubernetes API server, write them to a
// GitOps directory, or store them for tests.
type GatewayAPIObjectSink interface {
	ApplyGatewayAPIObjects(ctx context.Context, objects []KubernetesObject) error
}

// GatewayAPIOptions controls how generated gatewayx.Manifests are rendered into
// Gateway API resources.
type GatewayAPIOptions struct {
	Namespace        string
	GatewayName      string
	GatewayNamespace string
	HTTPSectionName  string
	GRPCSectionName  string
	Hostnames        []string
	RouteNamePrefix  string
	Labels           map[string]string
	Annotations      map[string]string

	// EmitAispherePolicies emits one AisphereRoutePolicy object per generated
	// route. These objects carry proto-derived access/authz/audit metadata that
	// plain Gateway API implementations do not understand.
	EmitAispherePolicies bool
}

// GatewayAPIPublisher renders route manifests into Kubernetes Gateway API
// objects and writes them through a GatewayAPIObjectSink.
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
	if len(objects) == 0 {
		return nil
	}
	return p.Sink.ApplyGatewayAPIObjects(ctx, objects)
}

// MemoryGatewayAPIObjectSink is useful for tests and local inspection.
type MemoryGatewayAPIObjectSink struct {
	Objects []KubernetesObject
}

func (s *MemoryGatewayAPIObjectSink) ApplyGatewayAPIObjects(ctx context.Context, objects []KubernetesObject) error {
	_ = ctx
	s.Objects = append(s.Objects, objects...)
	return nil
}

// RenderGatewayAPIObjects converts a gatewayx.Manifest into HTTPRoute,
// GRPCRoute, and optional AisphereRoutePolicy objects.
func RenderGatewayAPIObjects(manifest Manifest, opts GatewayAPIOptions) ([]KubernetesObject, error) {
	manifest = normalizeManifest(manifest)
	if manifest.Service == "" || len(manifest.Routes) == 0 {
		return nil, nil
	}
	opts = opts.normalized(manifest)

	var httpRules []map[string]any
	var grpcRules []map[string]any
	var policies []KubernetesObject
	for _, route := range manifest.Routes {
		if route.ID == "" {
			continue
		}
		if isGRPCRoute(route) {
			grpcRules = append(grpcRules, renderGRPCRule(route, opts))
		} else {
			httpRules = append(httpRules, renderHTTPRule(route, opts))
		}
		if opts.EmitAispherePolicies {
			policies = append(policies, renderAisphereRoutePolicy(route, opts))
		}
	}

	objects := make([]KubernetesObject, 0, 2+len(policies))
	if len(httpRules) > 0 {
		objects = append(objects, KubernetesObject{
			APIVersion: GatewayAPIVersion,
			Kind:       "HTTPRoute",
			Metadata:   opts.metadata(opts.routeName(manifest.Service, "http")),
			Spec: map[string]any{
				"parentRefs": []map[string]any{opts.parentRef(opts.HTTPSectionName)},
				"hostnames":  opts.Hostnames,
				"rules":      httpRules,
			},
		})
	}
	if len(grpcRules) > 0 {
		objects = append(objects, KubernetesObject{
			APIVersion: GatewayAPIVersion,
			Kind:       "GRPCRoute",
			Metadata:   opts.metadata(opts.routeName(manifest.Service, "grpc")),
			Spec: map[string]any{
				"parentRefs": []map[string]any{opts.parentRef(opts.GRPCSectionName)},
				"hostnames":  opts.Hostnames,
				"rules":      grpcRules,
			},
		})
	}
	objects = append(objects, policies...)
	return objects, nil
}

func renderHTTPRule(route GatewayRoute, opts GatewayAPIOptions) map[string]any {
	method := strings.ToUpper(strings.TrimSpace(route.Method))
	if method == "" {
		method = http.MethodGet
	}
	return map[string]any{
		"name": ruleName(route),
		"matches": []map[string]any{{
			"method": method,
			"path": map[string]any{
				"type":  "PathPrefix",
				"value": gatewayAPIPath(route.Path),
			},
		}},
		"backendRefs": []map[string]any{backendRef(route, opts, defaultHTTPPort(route))},
	}
}

func renderGRPCRule(route GatewayRoute, opts GatewayAPIOptions) map[string]any {
	service, method := grpcServiceMethod(route.Upstream.Operation)
	match := map[string]any{}
	if service != "" || method != "" {
		match["method"] = map[string]any{
			"service": service,
			"method":  method,
		}
	}
	if len(match) == 0 {
		match["method"] = map[string]any{}
	}
	return map[string]any{
		"name":        ruleName(route),
		"matches":     []map[string]any{match},
		"backendRefs": []map[string]any{backendRef(route, opts, defaultGRPCPort(route))},
	}
}

func renderAisphereRoutePolicy(route GatewayRoute, opts GatewayAPIOptions) KubernetesObject {
	labels := copyStringMap(opts.Labels)
	labels["aisphere.io/service"] = firstNonEmpty(route.Upstream.Service, "unknown")
	labels["aisphere.io/exposure"] = strings.ToLower(exposureName(route.Gateway.Exposure))
	return KubernetesObject{
		APIVersion: AispherePolicyVersion,
		Kind:       "AisphereRoutePolicy",
		Metadata: ObjectMeta{
			Name:      sanitizeName("policy-" + route.ID),
			Namespace: opts.Namespace,
			Labels:    labels,
		},
		Spec: map[string]any{
			"target": map[string]any{
				"operation": route.Upstream.Operation,
				"method":    route.Method,
				"path":      route.Path,
			},
			"exposure": exposureName(route.Gateway.Exposure),
			"authn": map[string]any{
				"mode": string(route.Gateway.EffectiveAuthnMode()),
			},
			"authz": map[string]any{
				"action":   route.Access.Action,
				"resource": route.Access.Resource,
				"audience": route.Access.Audience,
			},
		},
	}
}

func (o GatewayAPIOptions) normalized(manifest Manifest) GatewayAPIOptions {
	if o.Namespace == "" {
		o.Namespace = manifest.Namespace
	}
	if o.Namespace == "" {
		o.Namespace = "default"
	}
	if o.GatewayName == "" {
		o.GatewayName = "aisphere-public"
	}
	if len(o.Hostnames) == 0 {
		o.Hostnames = []string{"*"}
	}
	if o.HTTPSectionName == "" {
		o.HTTPSectionName = "http"
	}
	if o.GRPCSectionName == "" {
		o.GRPCSectionName = "grpc"
	}
	return o
}

func (o GatewayAPIOptions) metadata(name string) ObjectMeta {
	labels := copyStringMap(o.Labels)
	labels["app.kubernetes.io/managed-by"] = "aisphere-kernel"
	annotations := copyStringMap(o.Annotations)
	return ObjectMeta{Name: name, Namespace: o.Namespace, Labels: labels, Annotations: annotations}
}

func (o GatewayAPIOptions) routeName(service, protocol string) string {
	name := sanitizeName(firstNonEmpty(o.RouteNamePrefix, "") + service + "-" + protocol)
	if name == "" {
		return protocol + "-route"
	}
	return name
}

func (o GatewayAPIOptions) parentRef(sectionName string) map[string]any {
	ref := map[string]any{"name": o.GatewayName}
	if o.GatewayNamespace != "" {
		ref["namespace"] = o.GatewayNamespace
	}
	if sectionName != "" {
		ref["sectionName"] = sectionName
	}
	return ref
}

func backendRef(route GatewayRoute, opts GatewayAPIOptions, defaultPort int) map[string]any {
	upstream := route.Upstream
	name := upstream.Service
	if name == "" {
		name = route.ID
	}
	port := upstream.Port
	if port <= 0 {
		port = defaultPort
	}
	ref := map[string]any{"name": name, "port": port}
	if upstream.Namespace != "" && upstream.Namespace != opts.Namespace {
		ref["namespace"] = upstream.Namespace
	}
	return ref
}

func defaultHTTPPort(route GatewayRoute) int {
	if route.Upstream.Port > 0 {
		return route.Upstream.Port
	}
	return 8000
}

func defaultGRPCPort(route GatewayRoute) int {
	if route.Upstream.Port > 0 {
		return route.Upstream.Port
	}
	return 9000
}

func isGRPCRoute(route GatewayRoute) bool {
	protocol := strings.ToLower(strings.TrimSpace(route.Upstream.Protocol))
	return protocol == "grpc" && strings.TrimSpace(route.Upstream.Operation) != "" && strings.TrimSpace(route.Path) == ""
}

func grpcServiceMethod(operation string) (string, string) {
	operation = strings.Trim(operation, "/")
	if operation == "" {
		return "", ""
	}
	parts := strings.Split(operation, "/")
	if len(parts) != 2 {
		return operation, ""
	}
	return parts[0], parts[1]
}

func gatewayAPIPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	// Gateway API does not use google.api.http {name} syntax directly in route
	// values. For PathPrefix matching, trim variable segments to their stable
	// prefix so the backend/kernel still performs exact operation binding.
	if i := strings.Index(p, "{"); i >= 0 {
		p = strings.TrimRight(p[:i], "/")
		if p == "" {
			return "/"
		}
	}
	return p
}

func ruleName(route GatewayRoute) string {
	return sanitizeName(route.ID)
}

func sanitizeName(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	out := re.ReplaceAllString(in, "-")
	out = strings.Trim(out, "-")
	if len(out) > 63 {
		out = strings.Trim(out[:63], "-")
	}
	return out
}

func exposureName(exposure accessv1.Exposure) string {
	s := strings.TrimPrefix(exposure.String(), "Exposure_")
	if s == "EXPOSURE_UNSPECIFIED" || s == "" {
		return "AUTHENTICATED"
	}
	return s
}

func copyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
