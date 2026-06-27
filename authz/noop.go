package authz

import "context"

func DenyAll() Authorizer { return denyAllAuthorizer{} }

type denyAllAuthorizer struct{}

func (denyAllAuthorizer) Check(context.Context, CheckRequest) (Decision, error) {
	return Deny("deny_all"), nil
}

func AllowAllForDevOnly() Authorizer { return allowAllAuthorizer{} }

type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Check(context.Context, CheckRequest) (Decision, error) {
	return Allow("allow_all_dev_only"), nil
}
