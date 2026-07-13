package taskx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Option configures Scheduler.
type Option func(*Scheduler)

// WithLocker enables distributed singleton jobs.
func WithLocker(locker Locker) Option {
	return func(s *Scheduler) { s.locker = locker }
}

// WithObserver attaches logging, metrics, tracing, or audit adapters.
func WithObserver(observer Observer) Option {
	return func(s *Scheduler) {
		if observer != nil {
			s.observer = observer
		}
	}
}

// WithNow overrides the clock used for schedule calculations and tests.
func WithNow(now func() time.Time) Option {
	return func(s *Scheduler) {
		if now != nil {
			s.now = now
		}
	}
}

// Scheduler owns registered jobs and their lifecycle.
type Scheduler struct {
	mu       sync.RWMutex
	jobs     map[string]*runtimeJob
	locker   Locker
	observer Observer
	now      func() time.Time
	started  bool
	ctx      context.Context
	cancel   context.CancelFunc
	loopWG   sync.WaitGroup
	runWG    sync.WaitGroup
}

type runtimeJob struct {
	job     Job
	running atomic.Bool
}

// NewScheduler creates a scheduler. Register all jobs before Start.
func NewScheduler(opts ...Option) *Scheduler {
	s := &Scheduler{
		jobs:     make(map[string]*runtimeJob),
		observer: nopObserver(),
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register adds a job. Registration after Start is intentionally rejected to
// keep service boot deterministic.
func (s *Scheduler) Register(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrAlreadyStarted
	}
	if err := job.validate(s.now(), s.locker); err != nil {
		return err
	}
	if _, exists := s.jobs[job.Name]; exists {
		return fmt.Errorf("%w: %s", ErrJobAlreadyExists, job.Name)
	}
	job.Retry = job.Retry.normalized()
	if job.Lease.Enabled {
		job.Lease = job.Lease.normalized(job.Name, job.Timeout)
	}
	s.jobs[job.Name] = &runtimeJob{job: job}
	return nil
}

// Start begins scheduling and returns immediately.
func (s *Scheduler) Start(parent context.Context) error {
	if parent == nil {
		parent = context.Background()
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return ErrAlreadyStarted
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	s.started = true
	jobs := make([]*runtimeJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.Unlock()

	for _, job := range jobs {
		s.loopWG.Add(1)
		go s.runLoop(job)
	}
	return nil
}

// Shutdown stops scheduling, cancels active handlers, and waits for exit.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.RLock()
	started := s.started
	cancel := s.cancel
	s.mu.RUnlock()
	if !started {
		return ErrNotStarted
	}
	cancel()
	if err := waitGroup(ctx, &s.loopWG); err != nil {
		return err
	}
	return waitGroup(ctx, &s.runWG)
}

// Trigger executes a registered job immediately while preserving singleton,
// retry, timeout, lease, and observability behavior.
func (s *Scheduler) Trigger(ctx context.Context, name string) error {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.run(ctx, job, s.now())
}

func (s *Scheduler) runLoop(runtime *runtimeJob) {
	defer s.loopWG.Done()
	job := runtime.job
	anchor := s.now()
	if job.RunOnStart {
		s.launch(runtime, anchor)
	}

	next := job.Schedule.Next(anchor)
	for !next.IsZero() {
		if !next.After(anchor) {
			s.observe(s.ctx, Event{
				Kind:      EventFailed,
				JobName:   job.Name,
				Timestamp: s.now(),
				Reason:    "invalid_schedule",
				Err:       fmt.Errorf("%w: schedule returned %s after %s", ErrInvalidJob, next, anchor),
			})
			return
		}
		timer := time.NewTimer(maxDuration(0, next.Sub(s.now())))
		select {
		case <-s.ctx.Done():
			stopTimer(timer)
			return
		case <-timer.C:
			s.launch(runtime, next)
		}

		anchor = next
		next = job.Schedule.Next(anchor)
		now := s.now()
		for !next.IsZero() && !next.After(now) {
			anchor = next
			next = job.Schedule.Next(anchor)
		}
	}
}

func (s *Scheduler) launch(runtime *runtimeJob, scheduledAt time.Time) {
	ctx := s.ctx
	if ctx == nil || ctx.Err() != nil {
		return
	}
	s.runWG.Add(1)
	go func() {
		defer s.runWG.Done()
		_ = s.run(ctx, runtime, scheduledAt)
	}()
}

func (s *Scheduler) run(parent context.Context, runtime *runtimeJob, scheduledAt time.Time) error {
	job := runtime.job
	if !job.AllowOverlap && !runtime.running.CompareAndSwap(false, true) {
		err := errors.New("previous run is still active")
		s.observe(parent, Event{
			Kind:        EventSkipped,
			JobName:     job.Name,
			RunID:       newRunID(),
			ScheduledAt: scheduledAt,
			Timestamp:   s.now(),
			Reason:      "overlap",
			Err:         err,
		})
		return err
	}
	if !job.AllowOverlap {
		defer runtime.running.Store(false)
	}

	runID := newRunID()
	s.observe(parent, Event{
		Kind:        EventScheduled,
		JobName:     job.Name,
		RunID:       runID,
		ScheduledAt: scheduledAt,
		Timestamp:   s.now(),
		MaxAttempts: job.Retry.MaxAttempts,
	})

	runCtx, cancelRun := context.WithCancel(parent)
	var lease Lease
	defer func() {
		cancelRun()
		if lease != nil {
			releaseLease(lease)
		}
	}()

	var leaseErrCh chan error
	if job.Lease.Enabled {
		acquiredLease, acquired, err := s.locker.TryAcquire(runCtx, job.Lease.Key, job.Lease.TTL)
		if err != nil {
			s.observe(runCtx, Event{
				Kind:        EventFailed,
				JobName:     job.Name,
				RunID:       runID,
				ScheduledAt: scheduledAt,
				Timestamp:   s.now(),
				Reason:      "lease_acquire_failed",
				Err:         err,
			})
			return err
		}
		if !acquired {
			s.observe(runCtx, Event{
				Kind:        EventSkipped,
				JobName:     job.Name,
				RunID:       runID,
				ScheduledAt: scheduledAt,
				Timestamp:   s.now(),
				Reason:      "lease_contended",
				Err:         ErrLeaseNotAcquired,
			})
			return ErrLeaseNotAcquired
		}
		lease = acquiredLease
		leaseErrCh = make(chan error, 1)
		go renewLease(runCtx, cancelRun, lease, job.Lease, leaseErrCh)
	}

	startedAt := s.now()
	var finalErr error
	lastAttempt := 0
	for attempt := 1; attempt <= job.Retry.MaxAttempts; attempt++ {
		lastAttempt = attempt
		attemptStarted := s.now()
		s.observe(runCtx, Event{
			Kind:        EventStarted,
			JobName:     job.Name,
			RunID:       runID,
			ScheduledAt: scheduledAt,
			Timestamp:   attemptStarted,
			Attempt:     attempt,
			MaxAttempts: job.Retry.MaxAttempts,
		})

		attemptCtx := runCtx
		cancelAttempt := func() {}
		if job.Timeout > 0 {
			attemptCtx, cancelAttempt = context.WithTimeout(runCtx, job.Timeout)
		}
		err := invoke(attemptCtx, job.Handler)
		cancelAttempt()
		if err == nil && runCtx.Err() != nil {
			err = runCtx.Err()
		}
		if leaseErr := pollLeaseError(leaseErrCh); leaseErr != nil {
			err = errors.Join(err, leaseErr)
		}
		if err == nil {
			s.observe(runCtx, Event{
				Kind:        EventSucceeded,
				JobName:     job.Name,
				RunID:       runID,
				ScheduledAt: scheduledAt,
				Timestamp:   s.now(),
				Attempt:     attempt,
				MaxAttempts: job.Retry.MaxAttempts,
				Duration:    s.now().Sub(startedAt),
			})
			return nil
		}
		finalErr = err
		if attempt == job.Retry.MaxAttempts || runCtx.Err() != nil {
			break
		}

		backoff := job.Retry.backoff(attempt)
		s.observe(runCtx, Event{
			Kind:        EventRetrying,
			JobName:     job.Name,
			RunID:       runID,
			ScheduledAt: scheduledAt,
			Timestamp:   s.now(),
			Attempt:     attempt,
			MaxAttempts: job.Retry.MaxAttempts,
			Duration:    s.now().Sub(attemptStarted),
			Reason:      backoff.String(),
			Err:         err,
		})
		timer := time.NewTimer(backoff)
		select {
		case <-runCtx.Done():
			stopTimer(timer)
			finalErr = errors.Join(finalErr, runCtx.Err())
		case <-timer.C:
			continue
		}
		break
	}

	if leaseErr := pollLeaseError(leaseErrCh); leaseErr != nil {
		finalErr = errors.Join(finalErr, leaseErr)
	}
	reason := "attempts_exhausted"
	if runCtx.Err() != nil {
		reason = "cancelled"
	}
	s.observe(runCtx, Event{
		Kind:        EventFailed,
		JobName:     job.Name,
		RunID:       runID,
		ScheduledAt: scheduledAt,
		Timestamp:   s.now(),
		Attempt:     lastAttempt,
		MaxAttempts: job.Retry.MaxAttempts,
		Duration:    s.now().Sub(startedAt),
		Reason:      reason,
		Err:         finalErr,
	})
	return finalErr
}

func (s *Scheduler) observe(ctx context.Context, event Event) {
	defer func() { _ = recover() }()
	s.observer.Observe(ctx, event)
}

func renewLease(ctx context.Context, cancel context.CancelFunc, lease Lease, options LeaseOptions, errCh chan<- error) {
	ticker := time.NewTicker(options.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := lease.Renew(ctx, options.TTL); err != nil {
				select {
				case errCh <- errors.Join(ErrLeaseLost, err):
				default:
				}
				cancel()
				return
			}
		}
	}
}

func releaseLease(lease Lease) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = lease.Release(ctx)
}

func pollLeaseError(errCh <-chan error) error {
	if errCh == nil {
		return nil
	}
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func invoke(ctx context.Context, handler Handler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("taskx: handler panic: %v", recovered)
		}
	}()
	return handler(ctx)
}

func newRunID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

func waitGroup(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
