package gatewayx

import "fmt"

// StaticHosts is a validation/local substitute for Kubernetes Service DNS. In
// production, Gateway should resolve UpstreamRef through Kubernetes Service DNS
// or EndpointSlice, not through a custom registry.
type StaticHosts map[string]string

func (h StaticHosts) Resolve(upstream UpstreamRef) (string, error) {
	if h == nil {
		return "", fmt.Errorf("gatewayx: static hosts not configured")
	}
	if v := h[upstream.Key()]; v != "" {
		return v, nil
	}
	if upstream.Namespace == "" {
		if v := h[upstream.Service+".default"]; v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("gatewayx: upstream %s not found in static hosts", upstream.Key())
}
