package dapr

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeLifecycle struct {
	startErr    error
	gracefulErr error
	stopErr     error
	started     bool
	graceful    bool
	stopped     bool
	block       chan struct{}
}

func (f *fakeLifecycle) Start() error {
	f.started = true
	return f.startErr
}

func (f *fakeLifecycle) GracefulStop() error {
	f.graceful = true
	if f.block != nil {
		<-f.block
	}
	return f.gracefulErr
}

func (f *fakeLifecycle) Stop() error {
	f.stopped = true
	return f.stopErr
}

func TestCallbackServerLifecycle(t *testing.T) {
	lifecycle := &fakeLifecycle{}
	server := &CallbackServer{service: lifecycle}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !lifecycle.started {
		t.Fatal("Start() did not call service.Start")
	}
	if err := server.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !lifecycle.graceful {
		t.Fatal("Stop() did not call GracefulStop")
	}
}

func TestCallbackServerStopFallsBackOnDeadline(t *testing.T) {
	lifecycle := &fakeLifecycle{block: make(chan struct{})}
	server := &CallbackServer{service: lifecycle}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err := server.Stop(ctx)
	close(lifecycle.block)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() error = %v, want deadline exceeded", err)
	}
	if !lifecycle.stopped {
		t.Fatal("Stop() did not fall back to service.Stop")
	}
}
