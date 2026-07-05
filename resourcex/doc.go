// Package resourcex defines Kernel's provider-neutral resource control-plane
// contracts.
//
// The package is intentionally business-neutral. It does not know about
// Aisphere-specific resources such as skills, repositories, agents or
// sandboxes. Applications and platform services implement these interfaces in
// their own control-plane service, while shared middleware and generated code
// can depend on the stable contracts here.
package resourcex
