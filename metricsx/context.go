package metricsx

import "context"

type ctxManagerKey struct{}

// Inject stores a metrics manager in ctx. Passing nil stores a no-op manager so
// downstream code can always call metrics methods without nil checks.
func Inject(ctx context.Context, manager Manager) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxManagerKey{}, ensureManager(manager))
}

// FromContext returns the metrics manager stored in ctx, or a no-op manager if
// none is present. This is intended for bootstrap/lifecycle hooks that receive
// only context.Context from kernel.App.
func FromContext(ctx context.Context) Manager {
	if ctx == nil {
		return Noop()
	}
	if manager, ok := ctx.Value(ctxManagerKey{}).(Manager); ok {
		return ensureManager(manager)
	}
	return Noop()
}

// FromContextOr returns the metrics manager stored in ctx, or fallback when ctx
// has no manager. If fallback is nil, a no-op manager is returned.
func FromContextOr(ctx context.Context, fallback Manager) Manager {
	if ctx == nil {
		return ensureManager(fallback)
	}
	if manager, ok := ctx.Value(ctxManagerKey{}).(Manager); ok {
		return ensureManager(manager)
	}
	return ensureManager(fallback)
}

// Ensure returns manager when non-nil, otherwise a no-op Manager.
func Ensure(manager Manager) Manager {
	return ensureManager(manager)
}

func ensureManager(manager Manager) Manager {
	if manager == nil {
		return Noop()
	}
	return manager
}
