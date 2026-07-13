package taskx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTriggerRetriesUntilSuccess(t *testing.T) {
	var attempts atomic.Int32
	scheduler := NewScheduler()
	err := scheduler.Register(Job{
		Name:     "retry",
		Schedule: At(time.Now().Add(time.Hour)),
		Retry: RetryPolicy{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
		},
		Handler: func(context.Context) error {
			if attempts.Add(1) < 2 {
				return errors.New("transient")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Trigger(context.Background(), "retry"); err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestTriggerSkipsOverlappingRun(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	var once sync.Once

	scheduler := NewScheduler()
	if err := scheduler.Register(Job{
		Name:     "singleton",
		Schedule: At(time.Now().Add(time.Hour)),
		Handler: func(context.Context) error {
			once.Do(func() { close(started) })
			<-release
			return nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	go func() {
		firstDone <- scheduler.Trigger(context.Background(), "singleton")
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first run did not start")
	}

	if err := scheduler.Trigger(context.Background(), "singleton"); err == nil {
		t.Fatal("second Trigger() error = nil, want overlap error")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Trigger() error = %v", err)
	}
}

func TestRunOnStartAndShutdown(t *testing.T) {
	runs := make(chan struct{}, 1)
	scheduler := NewScheduler()
	if err := scheduler.Register(Job{
		Name:       "startup",
		Schedule:   Every(time.Hour),
		RunOnStart: true,
		Handler: func(context.Context) error {
			runs <- struct{}{}
			return nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-runs:
	case <-time.After(time.Second):
		t.Fatal("run-on-start job did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scheduler.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestTriggerRecoversHandlerPanic(t *testing.T) {
	scheduler := NewScheduler()
	if err := scheduler.Register(Job{
		Name:     "panic",
		Schedule: At(time.Now().Add(time.Hour)),
		Handler: func(context.Context) error {
			panic("boom")
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Trigger(context.Background(), "panic"); err == nil {
		t.Fatal("Trigger() error = nil, want recovered panic error")
	}
}

func TestRegisterRejectsInvalidInterval(t *testing.T) {
	scheduler := NewScheduler()
	err := scheduler.Register(Job{
		Name:     "invalid",
		Schedule: Every(0),
		Handler:  func(context.Context) error { return nil },
	})
	if !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("Register() error = %v, want ErrInvalidJob", err)
	}
}
