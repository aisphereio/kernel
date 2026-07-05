package accessx

// AccessConfig controls per-operation access policies. Each entry maps an
// operation pattern to its access mode. This allows operators to override
// the default authorization behavior without code changes.
//
// Example YAML:
//
//	security:
//	  access:
//	    skip_operations:
//	      - GetMe
//	      - CreateOrganization
//	    public_operations:
//	      - Login
//	      - Exchange
//	    allow_all_operations:   # deprecated, use skip_operations instead
//	      - GetMe
type AccessConfig struct {
	// SkipOperations lists operations that should skip the SpiceDB
	// authorization check but still require authentication and record audit.
	// Each entry can be a short method name (e.g. "GetMe"), a full gRPC
	// method name (e.g. "iam.v1.IAMAuthService/GetMe"), or an HTTP path
	// (e.g. "/v1/iam/control-plane/orgs").
	//
	// This is the recommended replacement for the deprecated AllowAllOperations.
	SkipOperations []string `json:"skip_operations" yaml:"skip_operations"`

	// PublicOperations lists operations that skip both authentication AND
	// authorization. Use for endpoints that must be accessible without any
	// credentials (health checks, login, token exchange, etc.). Guard.Require
	// still records audit with an anonymous actor.
	//
	// Each entry follows the same matching rules as SkipOperations.
	PublicOperations []string `json:"public_operations" yaml:"public_operations"`

	// AllowAllOperations lists operations where any authenticated user is
	// allowed. This is the legacy field — new deployments should use
	// SkipOperations instead.
	//
	// Deprecated: Use SkipOperations instead.
	AllowAllOperations []string `json:"allow_all_operations" yaml:"allow_all_operations"`
}
