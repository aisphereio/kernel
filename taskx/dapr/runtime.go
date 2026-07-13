package dapr

import (
	"context"
	"errors"
	"fmt"

	"github.com/aisphereio/kernel/taskx"
	daprclient "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprgrpc "github.com/dapr/go-sdk/service/grpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
)

// Client is the subset of the Dapr Go client used by taskx.
type Client interface {
	ScheduleJob(context.Context, *daprclient.Job) error
	GetJob(context.Context, string) (*daprclient.Job, error)
	DeleteJob(context.Context, string) error
	Close()
}

// CallbackRegistrar registers handlers on Dapr's AppCallback gRPC service.
type CallbackRegistrar interface {
	AddJobEventHandler(string, common.JobEventHandler) error
}

// Runtime implements taskx.Runtime using the Dapr Jobs gRPC API. The Dapr
// Scheduler persists definitions in etcd and dispatches due jobs to one sidecar
// replica for the application ID.
type Runtime struct {
	client    Client
	callbacks CallbackRegistrar
	owned     bool
}

var _ taskx.Runtime = (*Runtime)(nil)

// New creates a Dapr runtime from an existing Dapr client and callback
// registrar. This constructor is useful for tests and advanced boot wiring.
func New(client Client, callbacks CallbackRegistrar) (*Runtime, error) {
	if client == nil {
		return nil, errors.New("taskx/dapr: client is required")
	}
	if callbacks == nil {
		return nil, errors.New("taskx/dapr: callback registrar is required")
	}
	return &Runtime{client: client, callbacks: callbacks}, nil
}

// Attach registers Dapr's AppCallback and Jobs callback services on an existing
// Kernel gRPC server and creates a sidecar client using DAPR_GRPC_ENDPOINT or
// DAPR_GRPC_PORT. Kernel remains responsible for starting and stopping server.
func Attach(server *grpc.Server) (*Runtime, error) {
	if server == nil {
		return nil, errors.New("taskx/dapr: grpc server is required")
	}
	client, err := daprclient.NewClient()
	if err != nil {
		return nil, fmt.Errorf("taskx/dapr: create sidecar client: %w", err)
	}
	callbacks := daprgrpc.NewServiceWithGrpcServer(nil, server)
	return &Runtime{client: client, callbacks: callbacks, owned: true}, nil
}

// AttachWithClient is the explicit-client variant of Attach.
func AttachWithClient(server *grpc.Server, client Client) (*Runtime, error) {
	if server == nil {
		return nil, errors.New("taskx/dapr: grpc server is required")
	}
	if client == nil {
		return nil, errors.New("taskx/dapr: client is required")
	}
	callbacks := daprgrpc.NewServiceWithGrpcServer(nil, server)
	return &Runtime{client: client, callbacks: callbacks}, nil
}

func (r *Runtime) Schedule(ctx context.Context, spec taskx.ManagedJob) error {
	if err := spec.Validate(); err != nil {
		return err
	}

	job := &daprclient.Job{
		Name:      spec.Name,
		Overwrite: spec.Overwrite,
	}
	if spec.Schedule != "" {
		job.Schedule = ptr(spec.Schedule)
	}
	if spec.DueTime != "" {
		job.DueTime = ptr(spec.DueTime)
	}
	if spec.Repeats != nil {
		value := *spec.Repeats
		job.Repeats = &value
	}
	if spec.TTL != "" {
		job.TTL = ptr(spec.TTL)
	}
	if spec.Data != nil || spec.DataTypeURL != "" {
		job.Data = &anypb.Any{
			TypeUrl: spec.DataTypeURL,
			Value:   append([]byte(nil), spec.Data...),
		}
	}
	if spec.FailurePolicy != nil {
		switch spec.FailurePolicy.Mode {
		case taskx.DeliveryFailureConstant:
			policy := &daprclient.JobFailurePolicyConstant{
				MaxRetries: cloneUint32(spec.FailurePolicy.MaxRetries),
			}
			if spec.FailurePolicy.Interval > 0 {
				interval := spec.FailurePolicy.Interval
				policy.Interval = &interval
			}
			job.FailurePolicy = policy
		case taskx.DeliveryFailureDrop:
			job.FailurePolicy = &daprclient.JobFailurePolicyDrop{}
		}
	}

	if err := r.client.ScheduleJob(ctx, job); err != nil {
		return fmt.Errorf("taskx/dapr: schedule job %q: %w", spec.Name, err)
	}
	return nil
}

func (r *Runtime) Get(ctx context.Context, name string) (taskx.ManagedJob, error) {
	if name == "" {
		return taskx.ManagedJob{}, fmt.Errorf("%w: managed job name is required", taskx.ErrInvalidJob)
	}
	job, err := r.client.GetJob(ctx, name)
	if err != nil {
		return taskx.ManagedJob{}, fmt.Errorf("taskx/dapr: get job %q: %w", name, err)
	}

	spec := taskx.ManagedJob{
		Name:      job.Name,
		Overwrite: job.Overwrite,
	}
	if job.Schedule != nil {
		spec.Schedule = *job.Schedule
	}
	if job.DueTime != nil {
		spec.DueTime = *job.DueTime
	}
	if job.Repeats != nil {
		value := *job.Repeats
		spec.Repeats = &value
	}
	if job.TTL != nil {
		spec.TTL = *job.TTL
	}
	if job.Data != nil {
		spec.DataTypeURL = job.Data.TypeUrl
		spec.Data = append([]byte(nil), job.Data.Value...)
	}
	if job.FailurePolicy != nil {
		switch policy := job.FailurePolicy.(type) {
		case *daprclient.JobFailurePolicyConstant:
			spec.FailurePolicy = &taskx.DeliveryFailurePolicy{
				Mode:       taskx.DeliveryFailureConstant,
				MaxRetries: cloneUint32(policy.MaxRetries),
			}
			if policy.Interval != nil {
				spec.FailurePolicy.Interval = *policy.Interval
			}
		case *daprclient.JobFailurePolicyDrop:
			spec.FailurePolicy = &taskx.DeliveryFailurePolicy{Mode: taskx.DeliveryFailureDrop}
		}
	}
	return spec, nil
}

func (r *Runtime) Delete(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("%w: managed job name is required", taskx.ErrInvalidJob)
	}
	if err := r.client.DeleteJob(ctx, name); err != nil {
		return fmt.Errorf("taskx/dapr: delete job %q: %w", name, err)
	}
	return nil
}

func (r *Runtime) RegisterHandler(name string, handler taskx.EventHandler) error {
	if name == "" {
		return fmt.Errorf("%w: handler name is required", taskx.ErrInvalidJob)
	}
	if handler == nil {
		return fmt.Errorf("%w: handler is required for %q", taskx.ErrInvalidJob, name)
	}
	if err := r.callbacks.AddJobEventHandler(name, func(ctx context.Context, event *common.JobEvent) error {
		return handler(ctx, taskx.TriggerEvent{
			Name: event.JobType,
			Data: append([]byte(nil), event.Data...),
		})
	}); err != nil {
		return fmt.Errorf("taskx/dapr: register handler %q: %w", name, err)
	}
	return nil
}

// Close releases a client created by Attach. It does not stop the shared Kernel
// gRPC server and does not shut down the Dapr sidecar.
func (r *Runtime) Close() {
	if r == nil || !r.owned || r.client == nil {
		return
	}
	r.client.Close()
}

func ptr[T any](value T) *T { return &value }

func cloneUint32(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
