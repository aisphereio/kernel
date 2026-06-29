package contextx

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/servicecontextx"
)

// ServiceContext is Kernel's go-zero-style dependency handoff point. It is
// re-exported from contextx so generated handler/logic code has one stable
// import for request context values and application dependencies.
type ServiceContext = servicecontextx.Context

// NewServiceContext creates a new dependency container.
func NewServiceContext() *ServiceContext { return servicecontextx.New() }

// MustNewServiceContext creates a new dependency container and mirrors the
// go-zero bootstrap style used by generated services.
func MustNewServiceContext() *ServiceContext { return servicecontextx.MustNew() }

// WithServiceContext attaches the application service context to ctx. Use it at
// Gateway/BFF and worker boundaries when code receives only context.Context but
// still needs access to shared clients/stores/guards.
func WithServiceContext(ctx context.Context, sc *ServiceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, keyServiceContext, sc)
}

// ServiceContextFromContext returns the service context attached to ctx.
func ServiceContextFromContext(ctx context.Context) (*ServiceContext, bool) {
	if ctx == nil {
		return nil, false
	}
	sc, ok := ctx.Value(keyServiceContext).(*ServiceContext)
	return sc, ok && sc != nil
}

// PutDependency registers dep by concrete type in the service context.
func PutDependency(sc *ServiceContext, dep any) error { return sc.Put(dep) }

// MustPutDependency registers dep by concrete type and panics on programmer
// error. Prefer this during service startup.
func MustPutDependency(sc *ServiceContext, dep any) { sc.MustPut(dep) }

// GetDependency fetches a dependency by type from the service context.
func GetDependency[T any](sc *ServiceContext) (T, bool) { return servicecontextx.Get[T](sc) }

// MustGetDependency fetches a dependency by type or panics.
func MustGetDependency[T any](sc *ServiceContext) T { return servicecontextx.MustGet[T](sc) }

// GetDependencyFromContext fetches a dependency by type from the ServiceContext
// stored in ctx.
func GetDependencyFromContext[T any](ctx context.Context) (T, bool) {
	var zero T
	sc, ok := ServiceContextFromContext(ctx)
	if !ok {
		return zero, false
	}
	return servicecontextx.Get[T](sc)
}

// MustGetDependencyFromContext fetches a dependency from ctx or panics.
func MustGetDependencyFromContext[T any](ctx context.Context) T {
	v, ok := GetDependencyFromContext[T](ctx)
	if !ok {
		var zero T
		panic(fmt.Sprintf("contextx: missing dependency %T", zero))
	}
	return v
}
