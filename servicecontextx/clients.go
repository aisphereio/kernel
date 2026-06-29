package servicecontextx

import (
	"fmt"
	"time"
)

// ClientKind classifies a dependency that talks to another service. The value
// is intentionally string-based so projects can add custom kinds without a
// framework release.
type ClientKind string

const (
	ClientKindGRPC ClientKind = "grpc"
	ClientKindHTTP ClientKind = "http"
)

// ClientRef describes an internal service client stored in ServiceContext. It
// keeps the generated client value plus enough metadata for logs, health checks,
// and generated code to understand how the client was wired.
type ClientRef struct {
	Name     string
	Kind     ClientKind
	Target   string
	Client   any
	Timeout  time.Duration
	Metadata map[string]string
}

// PutClient stores a named internal service client. Generated Gateway/BFF logic
// should prefer named clients over constructing gRPC/HTTP clients inline.
func (c *Context) PutClient(ref ClientRef) error {
	if ref.Name == "" {
		return fmt.Errorf("servicecontextx: empty client name")
	}
	if ref.Client == nil {
		return fmt.Errorf("servicecontextx: nil client %q", ref.Name)
	}
	return c.PutAs("client:"+ref.Name, ref)
}

// MustPutClient stores a named client and panics on programmer error.
func (c *Context) MustPutClient(ref ClientRef) {
	if err := c.PutClient(ref); err != nil {
		panic(err)
	}
}

// Client returns a named client descriptor.
func Client(c *Context, name string) (ClientRef, bool) {
	return GetAs[ClientRef](c, "client:"+name)
}

// MustClient returns a named client descriptor or panics.
func MustClient(c *Context, name string) ClientRef {
	return MustGetAs[ClientRef](c, "client:"+name)
}

// TypedClient returns the generated client value for name.
func TypedClient[T any](c *Context, name string) (T, bool) {
	var zero T
	ref, ok := Client(c, name)
	if !ok {
		return zero, false
	}
	v, ok := ref.Client.(T)
	return v, ok
}

// MustTypedClient returns the generated client value for name or panics.
func MustTypedClient[T any](c *Context, name string) T {
	v, ok := TypedClient[T](c, name)
	if !ok {
		var zero T
		panic(fmt.Sprintf("servicecontextx: missing typed client %q as %T", name, zero))
	}
	return v
}
