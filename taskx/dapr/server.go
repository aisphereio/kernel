package dapr

import (
	"context"
	"errors"
	"fmt"

	daprclient "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprgrpc "github.com/dapr/go-sdk/service/grpc"
)

type serviceLifecycle interface {
	Start() error
	Stop() error
	GracefulStop() error
}

// CallbackServer adapts a standalone Dapr gRPC callback service to Kernel's
// transport.Server lifecycle. It is the recommended production mode when the
// primary application gRPC server has authn/authz middleware that must not be
// applied to trusted sidecar callbacks.
type CallbackServer struct {
	service serviceLifecycle
}

// NewStandalone creates a Dapr task Runtime and a dedicated gRPC callback
// server bound to callbackAddress. Register handlers before Kernel starts the
// returned CallbackServer.
//
// The Dapr sidecar app-port must point to callbackAddress. Calls from this
// Runtime to the sidecar still use DAPR_GRPC_ENDPOINT or DAPR_GRPC_PORT.
func NewStandalone(callbackAddress string) (*Runtime, *CallbackServer, error) {
	client, err := daprclient.NewClient()
	if err != nil {
		return nil, nil, fmt.Errorf("taskx/dapr: create sidecar client: %w", err)
	}
	callbacks, err := daprgrpc.NewService(callbackAddress)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("taskx/dapr: create callback server: %w", err)
	}
	runtime, err := newStandalone(client, callbacks)
	if err != nil {
		client.Close()
		_ = callbacks.Stop()
		return nil, nil, err
	}
	return runtime, &CallbackServer{service: callbacks}, nil
}

func newStandalone(client Client, callbacks common.Service) (*Runtime, error) {
	if client == nil {
		return nil, errors.New("taskx/dapr: client is required")
	}
	if callbacks == nil {
		return nil, errors.New("taskx/dapr: callback service is required")
	}
	return &Runtime{client: client, callbacks: callbacks, owned: true}, nil
}

// Start blocks while the Dapr callback gRPC service is serving.
func (s *CallbackServer) Start(context.Context) error {
	if s == nil || s.service == nil {
		return errors.New("taskx/dapr: callback service is required")
	}
	return s.service.Start()
}

// Stop gracefully shuts down the callback service, falling back to an immediate
// stop if the Kernel shutdown context expires.
func (s *CallbackServer) Stop(ctx context.Context) error {
	if s == nil || s.service == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan error, 1)
	go func() {
		done <- s.service.GracefulStop()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if err := s.service.Stop(); err != nil {
			return errors.Join(ctx.Err(), err)
		}
		return ctx.Err()
	}
}
