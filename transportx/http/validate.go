package http

import (
	"context"
	"sync"
)

// RequestValidator validates decoded request messages after path/query/body
// binding and before the generated handler calls middleware/business logic.
type RequestValidator interface {
	ValidateRequest(ctx context.Context, req any) error
}

// RequestValidatorFunc adapts a function into RequestValidator.
type RequestValidatorFunc func(ctx context.Context, req any) error

func (f RequestValidatorFunc) ValidateRequest(ctx context.Context, req any) error { return f(ctx, req) }

var requestValidatorState struct {
	mu sync.RWMutex
	v  RequestValidator
}

// SetRequestValidator installs a process-wide validator for protoc-gen-go-http
// generated handlers. Pass nil to disable validation.
func SetRequestValidator(v RequestValidator) {
	requestValidatorState.mu.Lock()
	defer requestValidatorState.mu.Unlock()
	requestValidatorState.v = v
}

// ValidateRequest invokes the currently configured validator. Generated
// go-http handlers call this after binding request values.
func ValidateRequest(ctx context.Context, req any) error {
	requestValidatorState.mu.RLock()
	v := requestValidatorState.v
	requestValidatorState.mu.RUnlock()
	if v == nil {
		return nil
	}
	return v.ValidateRequest(ctx, req)
}
