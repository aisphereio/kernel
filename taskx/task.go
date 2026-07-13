package taskx

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAlreadyStarted   = errors.New("taskx: scheduler already started")
	ErrNotStarted       = errors.New("taskx: scheduler not started")
	ErrJobAlreadyExists = errors.New("taskx: job already exists")
	ErrJobNotFound      = errors.New("taskx: job not found")
	ErrInvalidJob       = errors.New("taskx: invalid job")
	ErrLeaseNotAcquired = errors.New("taskx: distributed lease not acquired")
	ErrLeaseLost        = errors.New("taskx: distributed lease lost")
)

// Handler is a unit of background work. Implementations must be idempotent
// because retries and failover can result in at-least-once execution.
type Handler func(context.Context) error

// Schedule calculates the first execution strictly after the supplied time.
// Returning the zero time means that the schedule has completed.
type Schedule interface {
	Next(after time.Time) time.Time
}

type scheduleValidator interface {
	validate() error
}

// Job describes a scheduled background task.
type Job struct {
	Name         string
	Schedule     Schedule
	Handler      Handler
	RunOnStart   bool
	AllowOverlap bool
	Timeout      time.Duration
	Retry        RetryPolicy
	Lease        LeaseOptions
}

func (j Job) validate(now time.Time, locker Locker) error {
	if j.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidJob)
	}
	if j.Schedule == nil {
		return fmt.Errorf("%w: schedule is required for %q", ErrInvalidJob, j.Name)
	}
	if j.Handler == nil {
		return fmt.Errorf("%w: handler is required for %q", ErrInvalidJob, j.Name)
	}
	if j.Timeout < 0 {
		return fmt.Errorf("%w: timeout must be non-negative for %q", ErrInvalidJob, j.Name)
	}
	if validator, ok := j.Schedule.(scheduleValidator); ok {
		if err := validator.validate(); err != nil {
			return fmt.Errorf("%w: schedule for %q: %v", ErrInvalidJob, j.Name, err)
		}
	}
	next := j.Schedule.Next(now)
	if !next.IsZero() && !next.After(now) {
		return fmt.Errorf("%w: schedule for %q must return a time after its input", ErrInvalidJob, j.Name)
	}
	if err := j.Retry.validate(); err != nil {
		return fmt.Errorf("%w: retry policy for %q: %v", ErrInvalidJob, j.Name, err)
	}
	if err := j.Lease.validate(j.Timeout, locker); err != nil {
		return fmt.Errorf("%w: lease for %q: %v", ErrInvalidJob, j.Name, err)
	}
	return nil
}

// RetryPolicy controls retries for one scheduled run. MaxAttempts includes the
// first attempt. Zero values disable retries.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 1
	}
	if p.InitialBackoff <= 0 {
		p.InitialBackoff = time.Second
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = 30 * time.Second
	}
	if p.MaxBackoff < p.InitialBackoff {
		p.MaxBackoff = p.InitialBackoff
	}
	if p.Multiplier < 1 {
		p.Multiplier = 2
	}
	return p
}

func (p RetryPolicy) validate() error {
	if p.MaxAttempts < 0 {
		return errors.New("max attempts must be non-negative")
	}
	if p.InitialBackoff < 0 || p.MaxBackoff < 0 {
		return errors.New("backoff must be non-negative")
	}
	if p.Multiplier < 0 {
		return errors.New("multiplier must be non-negative")
	}
	return nil
}

func (p RetryPolicy) backoff(attempt int) time.Duration {
	p = p.normalized()
	backoff := float64(p.InitialBackoff)
	for i := 1; i < attempt; i++ {
		backoff *= p.Multiplier
		if time.Duration(backoff) >= p.MaxBackoff {
			return p.MaxBackoff
		}
	}
	return time.Duration(backoff)
}

// LeaseOptions enables cross-instance singleton execution.
type LeaseOptions struct {
	Enabled       bool
	Key           string
	TTL           time.Duration
	RenewInterval time.Duration
}

func (o LeaseOptions) normalized(jobName string, timeout time.Duration) LeaseOptions {
	if o.Key == "" {
		o.Key = jobName
	}
	if o.TTL <= 0 {
		if timeout > 0 {
			o.TTL = timeout + 30*time.Second
		} else {
			o.TTL = 5 * time.Minute
		}
	}
	if o.RenewInterval <= 0 {
		o.RenewInterval = o.TTL / 3
	}
	if o.RenewInterval < time.Second {
		o.RenewInterval = time.Second
	}
	return o
}

func (o LeaseOptions) validate(timeout time.Duration, locker Locker) error {
	if !o.Enabled {
		return nil
	}
	if locker == nil {
		return errors.New("locker is required when distributed lease is enabled")
	}
	if o.TTL < 0 || o.RenewInterval < 0 {
		return errors.New("ttl and renew interval must be non-negative")
	}
	o = o.normalized("job", timeout)
	if o.RenewInterval >= o.TTL {
		return errors.New("renew interval must be less than ttl")
	}
	return nil
}

// Locker coordinates a job run across service replicas.
type Locker interface {
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (lease Lease, acquired bool, err error)
}

// Lease is an owned distributed lock. Renew and Release must verify ownership.
type Lease interface {
	Renew(ctx context.Context, ttl time.Duration) error
	Release(ctx context.Context) error
}

// EventKind identifies scheduler lifecycle and execution events.
type EventKind string

const (
	EventScheduled EventKind = "scheduled"
	EventStarted   EventKind = "started"
	EventRetrying  EventKind = "retrying"
	EventSucceeded EventKind = "succeeded"
	EventFailed    EventKind = "failed"
	EventSkipped   EventKind = "skipped"
)

// Event is emitted for observability adapters.
type Event struct {
	Kind        EventKind
	JobName     string
	RunID       string
	ScheduledAt time.Time
	Timestamp   time.Time
	Attempt     int
	MaxAttempts int
	Duration    time.Duration
	Reason      string
	Err         error
}

// Observer receives task execution events. Implementations must be non-blocking
// or perform their own buffering.
type Observer interface {
	Observe(context.Context, Event)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(context.Context, Event)

func (f ObserverFunc) Observe(ctx context.Context, event Event) { f(ctx, event) }

func nopObserver() Observer { return ObserverFunc(func(context.Context, Event) {}) }
