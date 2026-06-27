// Package accessx provides a small authn + authz + audit facade for handlers.
//
// It is intentionally a thin orchestration layer. Business code may depend on
// Guard for endpoint protection, while lower-level services should still depend
// directly on authn.Authenticator, authz.Authorizer, and auditx.Recorder.
package accessx
