package accessx

import "strings"

// SkipPolicyResolver returns a SkipPolicy for a given operation name.
// This is the standard way to integrate config-driven skip policies
// into the middleware/access layer.
type SkipPolicyResolver func(operation string) SkipPolicy

// NewSkipPolicyResolver creates a SkipPolicyResolver from an AccessConfig.
// It supports matching against:
//   - Short method names (e.g. "GetMe")
//   - Full gRPC method names (e.g. "iam.v1.IAMAuthService/GetMe")
//   - Prefixed gRPC method names (e.g. "/iam.v1.IAMAuthService/GetMe")
//   - HTTP URL paths (e.g. "/v1/iam/control-plane/orgs")
//
// The resolver checks operations in this priority order:
// 1. PublicOperations — returns SkipAll (skip authn + authz)
// 2. SkipOperations — returns SkipAuthz (skip authz only)
// 3. AllowAllOperations (deprecated) — returns SkipAuthz (skip authz only)
// 4. Otherwise — returns SkipDefault (perform full check)
func NewSkipPolicyResolver(cfg AccessConfig) SkipPolicyResolver {
	// Build lookup sets for O(1) matching.
	publicSet := make(map[string]bool, len(cfg.PublicOperations))
	for _, op := range cfg.PublicOperations {
		publicSet[strings.TrimSpace(op)] = true
	}

	skipSet := make(map[string]bool, len(cfg.SkipOperations))
	for _, op := range cfg.SkipOperations {
		skipSet[strings.TrimSpace(op)] = true
	}

	// Legacy support: AllowAllOperations maps to SkipAuthz.
	allowAllSet := make(map[string]bool, len(cfg.AllowAllOperations))
	for _, op := range cfg.AllowAllOperations {
		allowAllSet[strings.TrimSpace(op)] = true
	}

	return func(operation string) SkipPolicy {
		// 1. Check public operations (skip everything).
		if matchOperation(publicSet, operation) {
			return SkipAll
		}
		// 2. Check skip operations (skip authz only).
		if matchOperation(skipSet, operation) {
			return SkipAuthz
		}
		// 3. Check legacy allow-all operations (skip authz only).
		if matchOperation(allowAllSet, operation) {
			return SkipAuthz
		}
		// 4. Default — perform full check.
		return SkipDefault
	}
}

// matchOperation checks whether an operation matches any entry in the set.
// It supports four matching strategies:
//  1. Exact match (full gRPC method or short name)
//  2. Short name match (extracts "GetMe" from "iam.v1.IAMAuthService/GetMe")
//  3. Prefix match (for wildcard patterns like "/grpc.health.v1.Health/*")
//  4. Stripped prefix match (removes leading "/" for matching)
func matchOperation(set map[string]bool, operation string) bool {
	if len(set) == 0 || operation == "" {
		return false
	}

	// 1. Exact match first (fast path).
	if set[operation] {
		return true
	}

	// 2. Try short name extraction from gRPC full method.
	//    "iam.v1.IAMAuthService/GetMe" -> "GetMe"
	//    "/iam.v1.IAMAuthService/GetMe" -> "GetMe"
	if idx := strings.LastIndex(operation, "/"); idx >= 0 {
		shortName := operation[idx+1:]
		if shortName != "" && set[shortName] {
			return true
		}
	}

	// 3. Try without leading "/" for matching against config entries
	//    that may or may not have the leading slash.
	trimmed := strings.TrimPrefix(operation, "/")
	if trimmed != operation && set[trimmed] {
		return true
	}

	// 4. Check for wildcard/prefix patterns.
	for pattern := range set {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(operation, prefix) {
				return true
			}
		}
	}

	return false
}

// IsSkipPolicy returns true when the given policy indicates some form of
// skipping (either SkipAuthz or SkipAll).
func IsSkipPolicy(p SkipPolicy) bool {
	return p == SkipAuthz || p == SkipAll
}