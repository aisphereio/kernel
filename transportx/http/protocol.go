package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ProtocolRequest is the structured request passed through Kernel middleware
// before a native HTTP protocol handler is invoked. Payload contains the
// protocol-specific descriptor used by request-info and access resolvers; the
// original request remains available to the terminal streaming handler.
type ProtocolRequest struct {
	Operation string
	Payload   any
	Request   *http.Request
}

// Validate verifies that a protocol request can be safely dispatched.
func (r ProtocolRequest) Validate() error {
	if strings.TrimSpace(r.Operation) == "" {
		return fmt.Errorf("transportx/http: protocol operation is required")
	}
	if r.Request == nil {
		return fmt.Errorf("transportx/http: protocol HTTP request is required")
	}
	return nil
}

// ProtocolDescriptor classifies an incoming native HTTP protocol request and
// returns the stable operation and structured payload consumed by middleware.
type ProtocolDescriptor func(*http.Request) (ProtocolRequest, error)

// HandleProtocol registers an exact native protocol route on the server.
func (s *Server) HandleProtocol(method, route string, descriptor ProtocolDescriptor, native http.Handler, filters ...FilterFunc) {
	s.Route("").HandleProtocol(method, route, descriptor, native, filters...)
}

// HandleProtocolPrefix registers a governed native protocol handler for every
// method below prefix. Unlike HandlePrefix, requests registered here always run
// through the Kernel service middleware chain.
func (s *Server) HandleProtocolPrefix(prefix string, descriptor ProtocolDescriptor, native http.Handler, filters ...FilterFunc) {
	clean := strings.Trim(strings.TrimSpace(prefix), "/")
	pattern := "/{protocol_path:.*}"
	if clean != "" {
		pattern = "/" + clean + "/{protocol_path:.*}"
	}
	s.HandleProtocol("*", pattern, descriptor, native, filters...)
}

// HandleProtocol registers a native HTTP protocol route that still runs
// through Kernel's service middleware chain. The descriptor executes at the
// transport boundary and supplies the structured payload consumed by the
// request-info and access resolvers. The terminal handler receives the
// middleware-enriched context without request or response buffering.
func (r *Router) HandleProtocol(method, relativePath string, descriptor ProtocolDescriptor, native http.Handler, filters ...FilterFunc) {
	r.Handle(method, relativePath, func(ctx Context) error {
		described, err := descriptor(ctx.Request())
		if err != nil {
			return err
		}
		if err := described.Validate(); err != nil {
			return err
		}

		SetOperation(ctx.Request().Context(), described.Operation)
		terminal := func(callCtx context.Context, _ any) (any, error) {
			native.ServeHTTP(ctx.Response(), described.Request.WithContext(callCtx))
			return nil, nil
		}
		_, err = ctx.Middleware(terminal)(described.Request.Context(), described.Payload)
		return err
	}, filters...)
}
