package dapr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aisphereio/kernel/taskx"
	daprclient "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	"google.golang.org/protobuf/types/known/anypb"
)

type fakeClient struct {
	scheduled *daprclient.Job
	get       *daprclient.Job
	deleted   string
	err       error
	closed    bool
}

func (f *fakeClient) ScheduleJobAlpha1(_ context.Context, job *daprclient.Job) error {
		f.scheduled = job
		return f.err
	}

func (f *fakeClient) GetJob(context.Context, string) (*daprclient.Job, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.get, nil
}

func (f *fakeClient) DeleteJob(_ context.Context, name string) error {
	f.deleted = name
	return f.err
}

func (f *fakeClient) Close() { f.closed = true }

type fakeCallbacks struct {
	name    string
	handler common.JobEventHandler
	err     error
}

func (f *fakeCallbacks) AddJobEventHandler(name string, handler common.JobEventHandler) error {
	f.name = name
	f.handler = handler
	return f.err
}

func TestScheduleMapsManagedJob(t *testing.T) {
	client := &fakeClient{}
	callbacks := &fakeCallbacks{}
	runtime, err := New(client, callbacks)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	retries := uint32(4)
	repeats := uint32(8)
	err = runtime.Schedule(context.Background(), taskx.ManagedJob{
		Name:        "grant-expiration-reconciler",
		Schedule:    "@every 5m",
		Repeats:     &repeats,
		TTL:         "24h",
		Data:        []byte(`{"batch_size":100}`),
		DataTypeURL: "application/json",
		Overwrite:   true,
		FailurePolicy: &taskx.DeliveryFailurePolicy{
			Mode:       taskx.DeliveryFailureConstant,
			MaxRetries: &retries,
			Interval:   3 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if client.scheduled == nil {
		t.Fatal("ScheduleJob() was not called")
	}
	if got := client.scheduled.Name; got != "grant-expiration-reconciler" {
		t.Fatalf("Name = %q", got)
	}
	if client.scheduled.Schedule == nil || *client.scheduled.Schedule != "@every 5m" {
		t.Fatalf("Schedule = %v", client.scheduled.Schedule)
	}
	if client.scheduled.Repeats == nil || *client.scheduled.Repeats != repeats {
		t.Fatalf("Repeats = %v", client.scheduled.Repeats)
	}
	if client.scheduled.Data == nil || string(client.scheduled.Data.Value) != `{"batch_size":100}` {
		t.Fatalf("Data = %v", client.scheduled.Data)
	}
	policy, ok := client.scheduled.FailurePolicy.(*daprclient.JobFailurePolicyConstant)
	if !ok {
		t.Fatalf("FailurePolicy = %T", client.scheduled.FailurePolicy)
	}
	if policy.MaxRetries == nil || *policy.MaxRetries != retries || policy.Interval == nil || *policy.Interval != 3*time.Second {
		t.Fatalf("constant policy = %+v", policy)
	}
}

func TestGetMapsDaprJob(t *testing.T) {
	retries := uint32(2)
	interval := 5 * time.Second
	schedule := "@daily"
	ttl := "48h"
	client := &fakeClient{get: &daprclient.Job{
		Name:     "daily-cleanup",
		Schedule: &schedule,
		TTL:      &ttl,
		Data: &anypb.Any{
			TypeUrl: "application/json",
			Value:   []byte(`{"scope":"expired"}`),
		},
		FailurePolicy: &daprclient.JobFailurePolicyConstant{
			MaxRetries: &retries,
			Interval:   &interval,
		},
	}}
	runtime, err := New(client, &fakeCallbacks{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	job, err := runtime.Get(context.Background(), "daily-cleanup")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if job.Name != "daily-cleanup" || job.Schedule != schedule || job.TTL != ttl {
		t.Fatalf("job = %+v", job)
	}
	if job.FailurePolicy == nil || job.FailurePolicy.MaxRetries == nil || *job.FailurePolicy.MaxRetries != retries {
		t.Fatalf("FailurePolicy = %+v", job.FailurePolicy)
	}
}

func TestRegisterHandlerAdaptsJobEvent(t *testing.T) {
	callbacks := &fakeCallbacks{}
	runtime, err := New(&fakeClient{}, callbacks)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var received taskx.TriggerEvent
	if err := runtime.RegisterHandler("grant-expiration-reconciler", func(_ context.Context, event taskx.TriggerEvent) error {
		received = event
		return nil
	}); err != nil {
		t.Fatalf("RegisterHandler() error = %v", err)
	}
	if callbacks.handler == nil {
		t.Fatal("callback was not registered")
	}
	if err := callbacks.handler(context.Background(), &common.JobEvent{
		JobType: "grant-expiration-reconciler",
		Data:    []byte("payload"),
	}); err != nil {
		t.Fatalf("callback error = %v", err)
	}
	if received.Name != "grant-expiration-reconciler" || string(received.Data) != "payload" {
		t.Fatalf("received = %+v", received)
	}
}

func TestProviderErrorsAreWrapped(t *testing.T) {
	providerErr := errors.New("sidecar unavailable")
	runtime, err := New(&fakeClient{err: providerErr}, &fakeCallbacks{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := runtime.Delete(context.Background(), "job"); !errors.Is(err, providerErr) {
		t.Fatalf("Delete() error = %v", err)
	}
}
