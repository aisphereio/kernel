package dtmx

import "context"

type ctxManagerKey struct{}

func Inject(ctx context.Context, manager Manager) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if manager == nil {
		manager = Disabled()
	}
	return context.WithValue(ctx, ctxManagerKey{}, manager)
}

func FromContext(ctx context.Context) Manager {
	if ctx == nil {
		return Disabled()
	}
	if manager, ok := ctx.Value(ctxManagerKey{}).(Manager); ok && manager != nil {
		return manager
	}
	return Disabled()
}

func FromContextOr(ctx context.Context, fallback Manager) Manager {
	if ctx == nil {
		if fallback == nil {
			return Disabled()
		}
		return fallback
	}
	if manager, ok := ctx.Value(ctxManagerKey{}).(Manager); ok && manager != nil {
		return manager
	}
	if fallback == nil {
		return Disabled()
	}
	return fallback
}
