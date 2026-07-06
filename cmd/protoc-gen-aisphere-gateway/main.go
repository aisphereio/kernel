package main

import (
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/aisphereio/kernel/internal/protooptions"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	showVersion = flag.Bool("version", false, "print version and exit")

	provider                = flag.String("provider", "envoy-gateway", "gateway provider: envoy-gateway")
	namespace               = flag.String("namespace", "aisphere", "namespace for generated route resources")
	gatewayName             = flag.String("gateway_name", "aisphere-public", "Gateway API Gateway name")
	gatewayNamespace        = flag.String("gateway_namespace", "aisphere-gateway", "Gateway API Gateway namespace")
	hostname                = flag.String("host", "*", "Gateway API hostname")
	authService             = flag.String("auth_service", "aisphere-iam", "Envoy ext_authz service name")
	authNamespace           = flag.String("auth_namespace", "aisphere", "Envoy ext_authz service namespace")
	authPort                = flag.String("auth_port", "9002", "Envoy ext_authz gRPC service port")
	defaultBackendHTTPPort  = flag.String("default_backend_http_port", "8000", "default backend HTTP port")
	defaultBackendGRPCPort  = flag.String("default_backend_grpc_port", "9000", "default backend gRPC port")
	externalBackendProtocol = flag.String("external_backend_protocol", "http", "external HTTP backend protocol: http or grpc_transcoded")
	descriptorSetPath       = flag.String("descriptor_set", "descriptor.pb", "descriptor set path used by grpc_transcoded mode")
)

const (
	exposurePublic        int32 = 1
	exposureAuthenticated int32 = 2
	exposureAuthorized    int32 = 3
	exposureInternal      int32 = 4
	exposureSystem        int32 = 5
)

type routeClass string

const (
	classInternal          routeClass = "internal"
	classExternalPublic    routeClass = "external_public"
	classExternalProtected routeClass = "external_protected"
)

type routeSpec struct {
	Name        string
	Class       routeClass
	ServiceName string
	ServiceFull string
	MethodName  string
	FullMethod  string
	HTTPMethod  string
	HTTPPath    string
	Exposure    int32
	Access      protooptions.AccessPolicy
}

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-aisphere-gateway %s\n", release)
		return
	}
	protogen.Options{ParamFunc: flag.CommandLine.Set}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, file := range gen.Files {
			if file.Generate {
				generateFile(gen, file)
			}
		}
		return nil
	})
}

func generateFile(gen *protogen.Plugin, file *protogen.File) {
	routes := collectRoutes(file)
	if len(routes) == 0 {
		return
	}
	byService := map[string][]routeSpec{}
	for _, route := range routes {
		byService[route.ServiceName] = append(byService[route.ServiceName], route)
	}
	services := make([]string, 0, len(byService))
	for service := range byService {
		services = append(services, service)
	}
	sort.Strings(services)
	for _, service := range services {
		routes := byService[service]
		writeServiceArtifacts(gen, file, service, routes)
	}
}

func collectRoutes(file *protogen.File) []routeSpec {
	var routes []routeSpec
	for _, svc := range file.Services {
		serviceName := defaultServiceName(string(svc.Desc.FullName()))
		for _, method := range svc.Methods {
			access, ok := accessPolicy(method)
			if !ok || access.Gateway.Publish == protooptions.GatewayPublishDisabled {
				continue
			}
			exposure := access.Exposure
			if exposure == 0 {
				exposure = exposureAuthenticated
			}
			class := classify(exposure)
			fullMethod := "/" + string(svc.Desc.FullName()) + "/" + string(method.Desc.Name())
			base := routeSpec{
				Name:        safeName(serviceName + "-" + string(method.Desc.Name())),
				Class:       class,
				ServiceName: serviceName,
				ServiceFull: string(svc.Desc.FullName()),
				MethodName:  string(method.Desc.Name()),
				FullMethod:  fullMethod,
				Exposure:    exposure,
				Access:      access,
			}
			if rule, ok := proto.GetExtension(method.Desc.Options(), annotations.E_Http).(*annotations.HttpRule); ok && rule != nil {
				for _, binding := range httpBindings(rule) {
					r := base
					r.HTTPMethod = binding.method
					r.HTTPPath = binding.path
					routes = append(routes, r)
				}
			} else {
				routes = append(routes, base)
			}
		}
	}
	return routes
}

func accessPolicy(method *protogen.Method) (protooptions.AccessPolicy, bool) {
	unknown := method.Desc.Options().ProtoReflect().GetUnknown()
	payload, ok := protooptions.LastExtensionPayload(unknown, protooptions.ExtAccess)
	if !ok {
		return protooptions.AccessPolicy{}, false
	}
	return protooptions.ParseAccessPolicy(payload), true
}

type binding struct{ method, path string }

func httpBindings(rule *annotations.HttpRule) []binding {
	var out []binding
	for _, r := range append([]*annotations.HttpRule{rule}, rule.AdditionalBindings...) {
		method, path := httpBinding(r)
		if path != "" {
			out = append(out, binding{method: method, path: path})
		}
	}
	return out
}

func httpBinding(rule *annotations.HttpRule) (string, string) {
	switch p := rule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		return http.MethodGet, p.Get
	case *annotations.HttpRule_Post:
		return http.MethodPost, p.Post
	case *annotations.HttpRule_Put:
		return http.MethodPut, p.Put
	case *annotations.HttpRule_Delete:
		return http.MethodDelete, p.Delete
	case *annotations.HttpRule_Patch:
		return http.MethodPatch, p.Patch
	case *annotations.HttpRule_Custom:
		return strings.ToUpper(p.Custom.Kind), p.Custom.Path
	default:
		return http.MethodPost, ""
	}
}

func classify(exposure int32) routeClass {
	switch exposure {
	case exposureInternal, exposureSystem:
		return classInternal
	case exposurePublic:
		return classExternalPublic
	default:
		return classExternalProtected
	}
}

func writeServiceArtifacts(gen *protogen.Plugin, file *protogen.File, service string, routes []routeSpec) {
	public := filterClass(routes, classExternalPublic)
	protected := filterClass(routes, classExternalProtected)
	if len(public) > 0 {
		writeHTTPRoute(gen, file, service, "public", public)
		writeGRPCRoute(gen, file, service, "public", public)
	}
	if len(protected) > 0 {
		writeHTTPRoute(gen, file, service, "protected", protected)
		writeGRPCRoute(gen, file, service, "protected", protected)
		writeSecurityPolicy(gen, file, service, protected)
		writeRoutePolicyMap(gen, file, service, protected)
	}
	if *externalBackendProtocol == "grpc_transcoded" {
		writeTranscoderConfig(gen, file, service, append(public, protected...))
	}
}

func filterClass(routes []routeSpec, class routeClass) []routeSpec {
	var out []routeSpec
	for _, route := range routes {
		if route.Class == class {
			out = append(out, route)
		}
	}
	return out
}

func writeHTTPRoute(gen *protogen.Plugin, file *protogen.File, service, group string, routes []routeSpec) {
	httpRoutes := make([]routeSpec, 0, len(routes))
	for _, r := range routes {
		if r.HTTPPath != "" {
			httpRoutes = append(httpRoutes, r)
		}
	}
	if len(httpRoutes) == 0 {
		return
	}
	name := service + "-" + group + "-http"
	g := gen.NewGeneratedFile("gateway/"+name+".yaml", file.GoImportPath)
	g.P("apiVersion: gateway.networking.k8s.io/v1")
	g.P("kind: HTTPRoute")
	g.P("metadata:")
	g.P("  name: ", name)
	g.P("  namespace: ", *namespace)
	g.P("  labels:")
	g.P("    app.kubernetes.io/managed-by: aisphere-kernel")
	g.P("    aisphere.io/route-class: external-", group)
	g.P("spec:")
	writeParentAndHosts(g)
	g.P("  rules:")
	for _, route := range httpRoutes {
		g.P("    - name: ", route.Name)
		g.P("      matches:")
		g.P("        - method: ", route.HTTPMethod)
		g.P("          path:")
		g.P("            type: PathPrefix")
		g.P("            value: ", quote(gatewayAPIPath(route.HTTPPath)))
		g.P("      backendRefs:")
		g.P("        - name: ", service)
		if *externalBackendProtocol == "grpc_transcoded" {
			g.P("          port: ", *defaultBackendGRPCPort)
		} else {
			g.P("          port: ", *defaultBackendHTTPPort)
		}
	}
}

func writeGRPCRoute(gen *protogen.Plugin, file *protogen.File, service, group string, routes []routeSpec) {
	name := service + "-" + group + "-grpc"
	g := gen.NewGeneratedFile("gateway/"+name+".yaml", file.GoImportPath)
	g.P("apiVersion: gateway.networking.k8s.io/v1")
	g.P("kind: GRPCRoute")
	g.P("metadata:")
	g.P("  name: ", name)
	g.P("  namespace: ", *namespace)
	g.P("  labels:")
	g.P("    app.kubernetes.io/managed-by: aisphere-kernel")
	g.P("    aisphere.io/route-class: external-", group)
	g.P("spec:")
	writeParentAndHosts(g)
	g.P("  rules:")
	for _, route := range routes {
		g.P("    - name: ", route.Name)
		g.P("      matches:")
		g.P("        - method:")
		g.P("            service: ", route.ServiceFull)
		g.P("            method: ", route.MethodName)
		g.P("      backendRefs:")
		g.P("        - name: ", service)
		g.P("          port: ", *defaultBackendGRPCPort)
	}
}

func writeParentAndHosts(g *protogen.GeneratedFile) {
	g.P("  parentRefs:")
	g.P("    - name: ", *gatewayName)
	if *gatewayNamespace != "" {
		g.P("      namespace: ", *gatewayNamespace)
	}
	g.P("  hostnames:")
	for _, host := range splitCSV(*hostname) {
		g.P("    - ", quote(host))
	}
}

func writeSecurityPolicy(gen *protogen.Plugin, file *protogen.File, service string, routes []routeSpec) {
	name := service + "-protected-extauth"
	g := gen.NewGeneratedFile("gateway/"+name+".yaml", file.GoImportPath)
	g.P("apiVersion: gateway.envoyproxy.io/v1alpha1")
	g.P("kind: SecurityPolicy")
	g.P("metadata:")
	g.P("  name: ", name)
	g.P("  namespace: ", *namespace)
	g.P("spec:")
	g.P("  targetRefs:")
	g.P("    - group: gateway.networking.k8s.io")
	g.P("      kind: HTTPRoute")
	g.P("      name: ", service, "-protected-http")
	g.P("    - group: gateway.networking.k8s.io")
	g.P("      kind: GRPCRoute")
	g.P("      name: ", service, "-protected-grpc")
	g.P("  extAuth:")
	g.P("    grpc:")
	g.P("      backendRefs:")
	g.P("        - name: ", *authService)
	if *authNamespace != "" && *authNamespace != *namespace {
		g.P("          namespace: ", *authNamespace)
	}
	g.P("          port: ", *authPort)
}

func writeRoutePolicyMap(gen *protogen.Plugin, file *protogen.File, service string, routes []routeSpec) {
	name := service + "-route-policy"
	g := gen.NewGeneratedFile("iam/"+name+".yaml", file.GoImportPath)
	g.P("apiVersion: v1")
	g.P("kind: ConfigMap")
	g.P("metadata:")
	g.P("  name: ", name)
	g.P("  namespace: ", *namespace)
	g.P("data:")
	g.P("  policy.yaml: |")
	g.P("    routes:")
	for _, route := range routes {
		g.P("      - name: ", quote(route.Name))
		g.P("        class: ", quote(string(route.Class)))
		g.P("        service: ", quote(route.ServiceFull))
		g.P("        operation: ", quote(route.FullMethod))
		if route.HTTPMethod != "" {
			g.P("        method: ", quote(route.HTTPMethod))
			g.P("        path: ", quote(route.HTTPPath))
			g.P("        match_prefix: ", quote(gatewayAPIPath(route.HTTPPath)))
		}
		g.P("        exposure: ", quote(exposureName(route.Exposure)))
		g.P("        authz:")
		g.P("          action: ", quote(route.Access.Authz.Action))
		g.P("          resource: ", quote(route.Access.Authz.Resource))
		g.P("          audience: ", quote(route.Access.Authz.Audience))
		g.P("          mode: ", quote(modeName(route.Access.Authz.Mode)))
		if route.Access.Audit.Enabled || route.Access.Audit.Event != "" {
			g.P("        audit:")
			g.P("          event: ", quote(route.Access.Audit.Event))
			g.P("          risk: ", quote(route.Access.Audit.Risk))
		}
	}
}

func writeTranscoderConfig(gen *protogen.Plugin, file *protogen.File, service string, routes []routeSpec) {
	if len(routes) == 0 {
		return
	}
	name := service + "-grpc-json-transcoder"
	g := gen.NewGeneratedFile("gateway/"+name+".yaml", file.GoImportPath)
	g.P("apiVersion: v1")
	g.P("kind: ConfigMap")
	g.P("metadata:")
	g.P("  name: ", name)
	g.P("  namespace: ", *namespace)
	g.P("  labels:")
	g.P("    app.kubernetes.io/managed-by: aisphere-kernel")
	g.P("    aisphere.io/backend-protocol: grpc-transcoded")
	g.P("data:")
	g.P("  transcoder.yaml: |")
	g.P("    descriptor_set: ", quote(*descriptorSetPath))
	g.P("    backend_protocol: grpc")
	g.P("    services:")
	seen := map[string]bool{}
	for _, route := range routes {
		if !seen[route.ServiceFull] {
			seen[route.ServiceFull] = true
			g.P("      - ", quote(route.ServiceFull))
		}
	}
	g.P("    note: ", quote("Envoy Gateway still needs an Envoy extension or patch policy to attach the grpc_json_transcoder filter."))
}

func defaultServiceName(full string) string {
	parts := strings.Split(full, ".")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, "Service")
	return "aisphere-" + safeName(name)
}

func safeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = strings.Trim(out[:63], "-")
	}
	if out == "" {
		return "route"
	}
	return out
}

func gatewayAPIPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if i := strings.Index(p, "{"); i >= 0 {
		p = strings.TrimRight(p[:i], "/")
		if p == "" {
			return "/"
		}
	}
	return p
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

func exposureName(v int32) string {
	switch v {
	case exposurePublic:
		return "PUBLIC"
	case exposureAuthenticated:
		return "AUTHENTICATED"
	case exposureAuthorized:
		return "AUTHORIZED"
	case exposureInternal:
		return "INTERNAL"
	case exposureSystem:
		return "SYSTEM"
	default:
		return "AUTHENTICATED"
	}
}

func modeName(v int32) string {
	switch v {
	case 1:
		return "CHECK_ONLY"
	case 2:
		return "SELF_CHECK"
	case 3:
		return "SKIP_AUTHZ"
	default:
		return "CHECK_ONLY"
	}
}
