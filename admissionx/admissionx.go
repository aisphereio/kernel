// Package admissionx implements Kubernetes-style mutating and validating admission chains.
// It is intended for framework-level policy hooks, not ad-hoc handler logic.
package admissionx

import (
	"context"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/requestx"
)

// Attributes are the normalized inputs passed to admission plugins.
type Attributes struct {
	Request requestx.Info
	Object  any
}

// MutatingPlugin may modify or default the request object before validation/business logic.
type MutatingPlugin interface {
	Name() string
	Admit(ctx context.Context, a Attributes) (any, error)
}

// ValidatingPlugin may reject the request after mutation and before business logic.
type ValidatingPlugin interface {
	Name() string
	Validate(ctx context.Context, a Attributes) error
}

// MutatingPluginFunc adapts a function to MutatingPlugin.
type MutatingPluginFunc struct {
	PluginName string
	Fn         func(context.Context, Attributes) (any, error)
}

func (f MutatingPluginFunc) Name() string {
	if f.PluginName != "" {
		return f.PluginName
	}
	return "mutating.func"
}
func (f MutatingPluginFunc) Admit(ctx context.Context, a Attributes) (any, error) {
	if f.Fn == nil {
		return a.Object, nil
	}
	return f.Fn(ctx, a)
}

// ValidatingPluginFunc adapts a function to ValidatingPlugin.
type ValidatingPluginFunc struct {
	PluginName string
	Fn         func(context.Context, Attributes) error
}

func (f ValidatingPluginFunc) Name() string {
	if f.PluginName != "" {
		return f.PluginName
	}
	return "validating.func"
}
func (f ValidatingPluginFunc) Validate(ctx context.Context, a Attributes) error {
	if f.Fn == nil {
		return nil
	}
	return f.Fn(ctx, a)
}

// Chain is the ordered admission chain.
type Chain struct {
	mutating   []MutatingPlugin
	validating []ValidatingPlugin
}

func New(mutating []MutatingPlugin, validating []ValidatingPlugin) Chain {
	return Chain{mutating: append([]MutatingPlugin(nil), mutating...), validating: append([]ValidatingPlugin(nil), validating...)}
}
func (c Chain) Empty() bool { return len(c.mutating) == 0 && len(c.validating) == 0 }

// Admit runs mutating plugins first, then validating plugins.
func (c Chain) Admit(ctx context.Context, obj any) (any, error) {
	info, _ := requestx.FromContext(ctx)
	cur := obj
	for _, p := range c.mutating {
		next, err := p.Admit(ctx, Attributes{Request: info, Object: cur})
		if err != nil {
			return nil, wrap("ADMISSION_MUTATION_DENIED", p.Name(), err)
		}
		if next != nil {
			cur = next
		}
	}
	for _, p := range c.validating {
		if err := p.Validate(ctx, Attributes{Request: info, Object: cur}); err != nil {
			return nil, wrap("ADMISSION_VALIDATION_DENIED", p.Name(), err)
		}
	}
	return cur, nil
}

// Middleware runs the admission chain before the business handler.
func Middleware(chain Chain) middleware.Middleware {
	if chain.Empty() {
		return func(next middleware.Handler) middleware.Handler { return next }
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			nextReq, err := chain.Admit(ctx, req)
			if err != nil {
				return nil, err
			}
			return next(ctx, nextReq)
		}
	}
}

func wrap(code, plugin string, err error) error {
	if err == nil {
		return nil
	}
	return errorx.BadRequest(errorx.Code(code), "admission plugin "+plugin+" rejected request", errorx.WithCause(err))
}
