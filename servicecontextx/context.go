// Package servicecontextx provides a tiny typed dependency container for
// generated or hand-written service entrypoints.
//
// It is inspired by go-zero's ServiceContext pattern: handlers/logic should not
// construct clients, stores or guards directly. Instead, application startup
// wires dependencies once and passes a ServiceContext through the route layer.
package servicecontextx

import (
	"fmt"
	"reflect"
	"sync"
)

// Context stores shared service dependencies by their concrete type or by a
// caller-supplied name.
//
// The container is intentionally small. It is not a full DI framework; it is a
// stable handoff point for generated route/logic code and for business modules
// that want go-zero-style dependency ownership without importing go-zero.
type Context struct {
	mu      sync.RWMutex
	byType  map[reflect.Type]any
	byName  map[string]any
	closers []func() error
}

// New creates an empty service context.
func New() *Context {
	return &Context{
		byType: make(map[reflect.Type]any),
		byName: make(map[string]any),
	}
}

// MustNew returns a service context and panics only if future construction
// options fail. It exists to mirror common service bootstrap style.
func MustNew() *Context { return New() }

// Put registers dep by its concrete dynamic type.
func (c *Context) Put(dep any) error {
	if dep == nil {
		return fmt.Errorf("servicecontextx: nil dependency")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byType[reflect.TypeOf(dep)] = dep
	return nil
}

// MustPut registers dep by concrete type and panics on programmer error.
func (c *Context) MustPut(dep any) {
	if err := c.Put(dep); err != nil {
		panic(err)
	}
}

// PutAs registers dep under a named key. Named dependencies are useful when a
// service has multiple values of the same concrete type, such as two RPC clients.
func (c *Context) PutAs(name string, dep any) error {
	if name == "" {
		return fmt.Errorf("servicecontextx: empty dependency name")
	}
	if dep == nil {
		return fmt.Errorf("servicecontextx: nil dependency %q", name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byName[name] = dep
	return nil
}

// MustPutAs registers dep under name and panics on programmer error.
func (c *Context) MustPutAs(name string, dep any) {
	if err := c.PutAs(name, dep); err != nil {
		panic(err)
	}
}

// Get returns a dependency by concrete type.
func Get[T any](c *Context) (T, bool) {
	var zero T
	if c == nil {
		return zero, false
	}
	t := reflect.TypeOf((*T)(nil)).Elem()
	c.mu.RLock()
	defer c.mu.RUnlock()
	dep, ok := c.byType[t]
	if !ok {
		return zero, false
	}
	v, ok := dep.(T)
	if !ok {
		return zero, false
	}
	return v, true
}

// MustGet returns a dependency by concrete type or panics. It should only be
// used during startup or in generated code where missing dependencies are
// programming errors.
func MustGet[T any](c *Context) T {
	v, ok := Get[T](c)
	if !ok {
		var zero T
		panic(fmt.Sprintf("servicecontextx: missing dependency %T", zero))
	}
	return v
}

// GetAs returns a named dependency.
func GetAs[T any](c *Context, name string) (T, bool) {
	var zero T
	if c == nil || name == "" {
		return zero, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	dep, ok := c.byName[name]
	if !ok {
		return zero, false
	}
	v, ok := dep.(T)
	if !ok {
		return zero, false
	}
	return v, true
}

// MustGetAs returns a named dependency or panics.
func MustGetAs[T any](c *Context, name string) T {
	v, ok := GetAs[T](c, name)
	if !ok {
		panic(fmt.Sprintf("servicecontextx: missing dependency %q", name))
	}
	return v
}

// OnClose registers a shutdown hook. Hooks run in reverse registration order.
func (c *Context) OnClose(fn func() error) {
	if c == nil || fn == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closers = append(c.closers, fn)
}

// Close executes registered shutdown hooks in reverse order and returns the
// first error, if any.
func (c *Context) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	closers := append([]func() error(nil), c.closers...)
	c.mu.Unlock()

	var first error
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i](); err != nil && first == nil {
			first = err
		}
	}
	return first
}
