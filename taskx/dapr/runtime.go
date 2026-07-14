package dapr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aisphereio/kernel/taskx"
	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
	daprclient "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprgrpc "github.com/dapr/go-sdk/service/grpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
)

// Client is the subset of the Dapr Go client used by taskx.
// It matches daprclient.GRPCClient exactly so any Dapr Go SDK version
// that implements these methods satisfies the interface.
type Client interface {
	ScheduleJob(context.Context, *daprclient.Job) error
	GetJob(context.Context, string) (*daprclient.Job, error)
	DeleteJob(context.Context, string) error
	Close()
}

// scheduleJobAlpha1 tries the gRPC ScheduleJobAlpha1 first, then falls back
// to the Dapr HTTP API (POST /v1.0/jobs/<name>). Dapr 1.17.x has the gRPC
// proto definition for ScheduleJobAlpha1 but the runtime does not register
// the handler, causing "failed to proxy request" errors.
func scheduleJobAlpha1(client Client, ctx context.Context, job *daprclient.Job) error {
	// Try gRPC ScheduleJobAlpha1 first (Dapr Go SDK v1.15+).
	if c, ok := client.(interface {
		ScheduleJobAlpha1(context.Context, *daprclient.Job) error
	}); ok {
		err := c.ScheduleJobAlpha1(ctx, job)
		if err == nil {
			return nil
		}
		// If the error is not the transparent proxy error, return it.
		// Otherwise fall through to HTTP API.
		if !isProxyError(err) {
			return err
		}
	}

	// Fallback: use Dapr HTTP API (POST /v1.0/jobs/<name>).
	return scheduleJobJSON(ctx, job)
}

// isProxyError checks if the error is the Dapr transparent proxy error
// that occurs when the sidecar does not recognize the gRPC method or when
// the request is misrouted as a service invocation (ERR_DIRECT_INVOKE).
func isProxyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed to proxy request") ||
		strings.Contains(msg, "dapr-callee-app-id") ||
		strings.Contains(msg, "dapr-app-id not found") ||
		strings.Contains(msg, "ERR_DIRECT_INVOKE") ||
		strings.Contains(msg, "failed getting app id")
}

// scheduleJobJSON calls the Dapr sidecar HTTP API to schedule a job.
// Dapr 1.17 exposes the Jobs API under the alpha1 prefix:
// POST http://localhost:<daprPort>/v1.0-alpha1/jobs/<name>
// Using /v1.0/jobs/<name> is interpreted as service invocation where "jobs"
// is treated as the target app-id, causing ERR_DIRECT_INVOKE.
func scheduleJobJSON(ctx context.Context, job *daprclient.Job) error {
	daprPort := os.Getenv("DAPR_HTTP_PORT")
	if daprPort == "" {
		daprPort = "3500"
	}

	// Build the request body matching Dapr Jobs HTTP API.
	// Dapr 1.17 requires the job name in the URL path only; including it in
	// the body causes DAPR_SCHEDULER_JOB_NAME error.
	body := map[string]any{
		"overwrite": job.Overwrite,
	}
	if job.Schedule != nil {
		body["schedule"] = *job.Schedule
	}
	if job.DueTime != nil {
		body["dueTime"] = *job.DueTime
	}
	if job.Repeats != nil {
		body["repeats"] = *job.Repeats
	}
	if job.TTL != nil {
		body["ttl"] = *job.TTL
	}
	if job.Data != nil {
		body["data"] = job.Data.Value
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("taskx/dapr: marshal job: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "POST",
		fmt.Sprintf("http://127.0.0.1:%s/v1.0-alpha1/jobs/%s", daprPort, job.Name),
		bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("taskx/dapr: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("taskx/dapr: http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("taskx/dapr: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
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

// AttachTransport registers Dapr callbacks on a Kernel transportx/grpc Server.
// It must be called after the server is constructed and before Start/Serve.
func AttachTransport(server *kgrpc.Server) (*Runtime, error) {
	if server == nil || server.Server == nil {
		return nil, errors.New("taskx/dapr: Kernel grpc server is required")
	}
	return Attach(server.Server)
}

// AttachTransportWithClient is the explicit-client variant of AttachTransport.
func AttachTransportWithClient(server *kgrpc.Server, client Client) (*Runtime, error) {
	if server == nil || server.Server == nil {
		return nil, errors.New("taskx/dapr: Kernel grpc server is required")
	}
	return AttachWithClient(server.Server, client)
}

// Attach registers Dapr's AppCallback and Jobs callback services on an existing
// gRPC server and creates a sidecar client using DAPR_GRPC_ENDPOINT or
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

// Use ScheduleJobAlpha1 when available (Dapr Runtime < 1.18 compat).
		// Dapr 1.17.x does not recognize the stable ScheduleJob gRPC method
		// and routes it to the transparent proxy, causing
		// "dapr-callee-app-id not found".
		if err := scheduleJobAlpha1(r.client, ctx, job); err != nil {
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
		if event == nil {
			return errors.New("taskx/dapr: nil job event")
		}
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
