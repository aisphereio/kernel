package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aisphereio/kernel/middleware"
)

func TestProtocolRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		request ProtocolRequest
		wantErr bool
	}{
		{
			name: "valid",
			request: ProtocolRequest{
				Operation: "/git.v1.Protocol/Fetch",
				Payload:   struct{ Repository string }{Repository: "demo"},
				Request:   httptest.NewRequest("POST", "/demo.git/git-upload-pack", nil),
			},
		},
		{
			name: "missing operation",
			request: ProtocolRequest{
				Request: httptest.NewRequest("POST", "/demo.git/git-upload-pack", nil),
			},
			wantErr: true,
		},
		{
			name: "missing request",
			request: ProtocolRequest{
				Operation: "/git.v1.Protocol/Fetch",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestProtocolDescriptorReturnsStructuredPayload(t *testing.T) {
	descriptor := ProtocolDescriptor(func(r *http.Request) (ProtocolRequest, error) {
		return ProtocolRequest{
			Operation: "/git.v1.Protocol/Fetch",
			Payload:   r.URL.Path,
			Request:   r,
		}, nil
	})

	req := httptest.NewRequest("GET", "/demo.git/info/refs", nil)
	described, err := descriptor(req)
	if err != nil {
		t.Fatalf("descriptor() error = %v", err)
	}
	if got, want := described.Payload, "/demo.git/info/refs"; got != want {
		t.Fatalf("Payload = %v, want %v", got, want)
	}
}

type protocolPayload struct {
	Repository string
}

type protocolContextKey struct{}

func TestHandleProtocolRunsMiddlewareBeforeStreamingHandler(t *testing.T) {
	payload := protocolPayload{Repository: "demo"}
	descriptor := func(r *http.Request) (ProtocolRequest, error) {
		return ProtocolRequest{
			Operation: "/git.v1.Protocol/Fetch",
			Payload:   payload,
			Request:   r,
		}, nil
	}

	var middlewareCalled bool
	protocolMiddleware := func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			got, ok := req.(protocolPayload)
			if !ok {
				t.Fatalf("middleware request type = %T, want protocolPayload", req)
			}
			if got != payload {
				t.Fatalf("middleware payload = %#v, want %#v", got, payload)
			}
			middlewareCalled = true
			return next(context.WithValue(ctx, protocolContextKey{}, "authorized"), req)
		}
	}

	srv := NewServer(Middleware(protocolMiddleware))
	srv.Route("").HandleProtocol(http.MethodPost, "/{repo}.git/git-upload-pack", descriptor, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value(protocolContextKey{}); got != "authorized" {
			t.Fatalf("handler context value = %v, want authorized", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("chunk-1"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}
		_, _ = w.Write([]byte("-chunk-2"))
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/demo.git/git-upload-pack", nil)
	srv.ServeHTTP(recorder, req)

	if !middlewareCalled {
		t.Fatal("middleware was not called")
	}
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := recorder.Body.String(), "chunk-1-chunk-2"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHandleProtocolStopsBeforeHandlerOnMiddlewareError(t *testing.T) {
	wantErr := errors.New("denied")
	deny := func(next middleware.Handler) middleware.Handler {
		return func(context.Context, any) (any, error) {
			return nil, wantErr
		}
	}

	descriptor := func(r *http.Request) (ProtocolRequest, error) {
		return ProtocolRequest{
			Operation: "/git.v1.Protocol/Push",
			Payload:   protocolPayload{Repository: "demo"},
			Request:   r,
		}, nil
	}

	var handlerCalled bool
	srv := NewServer(Middleware(deny))
	srv.Route("").HandleProtocol(http.MethodPost, "/{repo}.git/git-receive-pack", descriptor, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		handlerCalled = true
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/demo.git/git-receive-pack", nil)
	srv.ServeHTTP(recorder, req)

	if handlerCalled {
		t.Fatal("native handler ran after middleware denial")
	}
	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestHandleProtocolStopsBeforeHandlerOnDescriptorError(t *testing.T) {
	wantErr := errors.New("cannot classify request")
	var handlerCalled bool
	srv := NewServer()
	srv.Route("").HandleProtocol(http.MethodGet, "/{repo}.git/info/refs", func(*http.Request) (ProtocolRequest, error) {
		return ProtocolRequest{}, wantErr
	}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		handlerCalled = true
	}))

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/demo.git/info/refs", nil))

	if handlerCalled {
		t.Fatal("native handler ran after descriptor error")
	}
	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestServerHandleProtocolRegistersExactNativeRoute(t *testing.T) {
	var describedPath string
	var handledPath string
	srv := NewServer()
	srv.HandleProtocol(http.MethodGet, "/{repo:.*}.git/info/refs", func(r *http.Request) (ProtocolRequest, error) {
		describedPath = r.URL.Path
		return ProtocolRequest{
			Operation: "/git.v1.Protocol/Fetch",
			Payload:   protocolPayload{Repository: "demo"},
			Request:   r,
		}, nil
	}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handledPath = r.URL.Path
	}))

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/demo.git/info/refs", nil))

	if got, want := describedPath, "/demo.git/info/refs"; got != want {
		t.Fatalf("descriptor path = %q, want %q", got, want)
	}
	if got, want := handledPath, "/demo.git/info/refs"; got != want {
		t.Fatalf("handler path = %q, want %q", got, want)
	}
}

func TestServerHandleProtocolPrefixMatchesAllMethods(t *testing.T) {
	var handled []string
	srv := NewServer()
	srv.HandleProtocolPrefix("/git/", func(r *http.Request) (ProtocolRequest, error) {
		return ProtocolRequest{
			Operation: "/git.v1.Protocol/Native",
			Payload:   protocolPayload{Repository: "demo"},
			Request:   r,
		}, nil
	}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handled = append(handled, r.Method+" "+r.URL.Path)
	}))

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/git/demo.git/info/refs"},
		{method: http.MethodPost, path: "/git/demo.git/git-upload-pack"},
	} {
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, got, want)
		}
	}

	if got, want := len(handled), 2; got != want {
		t.Fatalf("handled count = %d, want %d", got, want)
	}
}
