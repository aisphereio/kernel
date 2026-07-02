package gatewayx

import (
	"path"
	"strings"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
)

// RouteFilter decides which generated routes are published to a concrete
// gateway profile. It is intentionally applied at registration time, not only
// at runtime, so public route registries do not leak internal/debug routes.
type RouteFilter struct {
	IncludeExposures []accessv1.Exposure
	ExcludeExposures []accessv1.Exposure

	IncludeProfiles []string
	ExcludeProfiles []string
	IncludeTags     []string
	ExcludeTags     []string

	IncludeServices []string
	ExcludeServices []string
	IncludePathGlobs []string
	ExcludePathGlobs []string
	IncludeIDs      []string
	ExcludeIDs      []string
}

// PublicRouteFilter publishes normal product APIs to a public gateway. INTERNAL
// and SYSTEM routes are deliberately excluded even if a service generated them.
func PublicRouteFilter() RouteFilter {
	return RouteFilter{
		IncludeExposures: []accessv1.Exposure{
			accessv1.Exposure_PUBLIC,
			accessv1.Exposure_AUTHENTICATED,
			accessv1.Exposure_AUTHORIZED,
		},
		ExcludeExposures: []accessv1.Exposure{
			accessv1.Exposure_INTERNAL,
			accessv1.Exposure_SYSTEM,
		},
		ExcludePathGlobs: []string{"/internal/*", "/debug/*", "/metrics", "/healthz", "/readyz"},
	}
}

// InternalRouteFilter publishes internal service-to-service APIs. It still
// excludes debug/ops-only endpoints by path because those should normally be
// served through an ops plane or direct probe, not service API dispatch.
func InternalRouteFilter() RouteFilter {
	return RouteFilter{
		IncludeExposures: []accessv1.Exposure{
			accessv1.Exposure_AUTHENTICATED,
			accessv1.Exposure_AUTHORIZED,
			accessv1.Exposure_INTERNAL,
			accessv1.Exposure_SYSTEM,
		},
		ExcludePathGlobs: []string{"/debug/*", "/metrics", "/healthz", "/readyz"},
	}
}

// OpsRouteFilter is for operator-only endpoints. Keep it opt-in; services
// should still avoid putting dangerous repair/migration/debug APIs in ordinary
// product manifests.
func OpsRouteFilter() RouteFilter {
	return RouteFilter{IncludeProfiles: []string{"ops"}}
}

func (f RouteFilter) Allow(route GatewayRoute) bool {
	if len(f.IncludeIDs) > 0 && !containsString(f.IncludeIDs, route.ID) {
		return false
	}
	if containsString(f.ExcludeIDs, route.ID) {
		return false
	}
	if len(f.IncludeServices) > 0 && !containsString(f.IncludeServices, route.Upstream.Service) {
		return false
	}
	if containsString(f.ExcludeServices, route.Upstream.Service) {
		return false
	}
	if len(f.IncludeExposures) > 0 && !containsExposure(f.IncludeExposures, route.Gateway.Exposure) {
		return false
	}
	if containsExposure(f.ExcludeExposures, route.Gateway.Exposure) {
		return false
	}
	if len(f.IncludeProfiles) > 0 && !intersectsString(f.IncludeProfiles, route.Gateway.Profiles) {
		return false
	}
	if intersectsString(f.ExcludeProfiles, route.Gateway.Profiles) {
		return false
	}
	if len(f.IncludeTags) > 0 && !intersectsString(f.IncludeTags, route.Gateway.Tags) {
		return false
	}
	if intersectsString(f.ExcludeTags, route.Gateway.Tags) {
		return false
	}
	if len(f.IncludePathGlobs) > 0 && !matchesAnyGlob(f.IncludePathGlobs, route.Path) {
		return false
	}
	if matchesAnyGlob(f.ExcludePathGlobs, route.Path) {
		return false
	}
	return true
}

func FilterManifest(manifest Manifest, filter RouteFilter) Manifest {
	manifest = normalizeManifest(manifest)
	out := Manifest{Service: manifest.Service, Namespace: manifest.Namespace}
	for _, route := range manifest.Routes {
		if filter.Allow(route) {
			out.Routes = append(out.Routes, route)
		}
	}
	return out
}

func containsExposure(list []accessv1.Exposure, target accessv1.Exposure) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func containsString(list []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, v := range list {
		if strings.TrimSpace(v) == target {
			return true
		}
	}
	return false
}

func intersectsString(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if strings.TrimSpace(x) != "" && strings.TrimSpace(x) == strings.TrimSpace(y) {
				return true
			}
		}
	}
	return false
}

func matchesAnyGlob(patterns []string, value string) bool {
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		ok, err := path.Match(p, value)
		if err == nil && ok {
			return true
		}
		// path.Match treats /internal/* as one segment. For HTTP prefix-style
		// routes, also support a simple suffix-star prefix match.
		if strings.HasSuffix(p, "*") && strings.HasPrefix(value, strings.TrimSuffix(p, "*")) {
			return true
		}
	}
	return false
}
