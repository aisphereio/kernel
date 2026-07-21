package kubernetesx

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
)

// VersionInfo is the stable, reduced view of a Kubernetes server version. It
// drops build/git fields that callers should not depend on and keeps the
// Major/Minor/GitVersion/Platform used by probe and capability checks.
type VersionInfo struct {
	Major      string `json:"major"`
	Minor      string `json:"minor"`
	GitVersion string `json:"git_version"`
	Platform   string `json:"platform"`
}

// FromVersionInfo converts a k8s.io/apimachinery version.Info into the stable
// kubernetesx.VersionInfo.
func FromVersionInfo(v *version.Info) VersionInfo {
	if v == nil {
		return VersionInfo{}
	}
	return VersionInfo{
		Major:      v.Major,
		Minor:      v.Minor,
		GitVersion: v.GitVersion,
		Platform:   v.Platform,
	}
}

// APIResource describes a single API resource kind discovered on the server.
type APIResource struct {
	Group     string   `json:"group"`
	Version   string   `json:"version"`
	Kind      string   `json:"kind"`
	Namespaced bool     `json:"namespaced"`
	Verbs     []string `json:"verbs"`
}

// Capabilities is the aggregated discovery result returned by Discover. It is
// the data Hub uses to decide whether a cluster can host a given CRD or run a
// given workload kind.
type Capabilities struct {
	ServerVersion VersionInfo   `json:"server_version"`
	APIs          []APIResource `json:"apis"`
}

// discover runs ServerVersion + ServerGroupsAndResources against the supplied
// discovery client and folds the result into Capabilities. It is shared by
// Client.Discover and Client.Probe.
func discover(ctx context.Context, d discoveryClient) (Capabilities, error) {
	// DiscoveryInterface.ServerVersion does not accept a context in
	// client-go v0.36; the per-request timeout is enforced by the rest
	// client configured on the discovery client. We honor ctx cancellation
	// for the groups/resources call where a context-bearing helper exists.
	select {
	case <-ctx.Done():
		return Capabilities{}, NormalizeError(ctx.Err())
	default:
	}
	info, err := d.ServerVersion()
	if err != nil {
		return Capabilities{}, NormalizeError(err)
	}
	_, resourceLists, err := d.ServerGroupsAndResources()
	if err != nil {
		// Partial discovery is still useful for probe; surface the error
		// but include whatever resources we got.
		apis := flattenResources(resourceLists)
		return Capabilities{
			ServerVersion: FromVersionInfo(info),
			APIs:          apis,
		}, NormalizeError(err)
	}
	return Capabilities{
		ServerVersion: FromVersionInfo(info),
		APIs:          flattenResources(resourceLists),
	}, nil
}

// discoveryClient is the subset of k8s.io/client-go/discovery.DiscoveryInterface
// that discover/probe need. Narrowing the interface here keeps the package
// testable with a hand-written fake without pulling the full discovery surface.
type discoveryClient interface {
	ServerVersion() (*version.Info, error)
	ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error)
}

// flattenResources converts discovery's nested APIResourceList (grouped by
// group/version) into a flat slice of APIResource with group/version/kind
// resolved. Resources with no verbs (e.g. subresources) are skipped.
func flattenResources(lists []*metav1.APIResourceList) []APIResource {
	return FlattenResources(lists)
}

// FlattenResources is the exported form of flattenResources. It lets the fake
// subpackage and external test doubles reuse the same flattening logic.
func FlattenResources(lists []*metav1.APIResourceList) []APIResource {
	var out []APIResource
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			if len(r.Verbs) == 0 {
				continue
			}
			out = append(out, APIResource{
				Group:      gv.Group,
				Version:    gv.Version,
				Kind:       r.Kind,
				Namespaced: r.Namespaced,
				Verbs:      append([]string(nil), r.Verbs...),
			})
		}
	}
	return out
}

// HasAPI reports whether capabilities advertise a resource matching the
// supplied group/version/kind. Used by probe to gate the Namespace access
// review on clusters that may not expose core/v1, and by callers to check
// whether a cluster can host a given CRD or workload kind.
func (c Capabilities) HasAPI(group, version, kind string) bool {
	for _, a := range c.APIs {
		if a.Group == group && a.Version == version && strings.EqualFold(a.Kind, kind) {
			return true
		}
	}
	return false
}

// hasAPI is an unexported alias kept for internal call sites.
func (c Capabilities) hasAPI(group, version, kind string) bool {
	return c.HasAPI(group, version, kind)
}
