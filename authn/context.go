package authn

import "context"

type principalContextKey struct{}

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal.Normalize())
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok || !principal.IsAuthenticated() {
		return Principal{}, false
	}
	return principal, true
}

func MustPrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, ErrUnauthenticated("")
	}
	return principal, nil
}
