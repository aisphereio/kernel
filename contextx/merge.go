package contextx

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// mergeCtx merges two parent contexts. The merged context is done when EITHER
// parent is done (whichever fires first), or when the returned CancelFunc is
// called. Values from parent1 take precedence over parent2.
//
// This is useful when a child goroutine needs values from a parent request
// context but should also be cancellable independently.
type mergeCtx struct {
	parent1, parent2 context.Context

	done     chan struct{}
	doneMark atomic.Bool
	doneOnce sync.Once
	doneErr  error

	cancelCh   chan struct{}
	cancelOnce sync.Once
}

// Merge merges two contexts into one. The merged context:
//   - Is done when EITHER parent is done, OR when the returned CancelFunc is called
//   - Returns the earliest of the two parents' deadlines (if both have one)
//   - Looks up values from parent1 first, then parent2
//
// The returned CancelFunc must be called to release the internal goroutine.
// Calling it after the merged context is already done is a no-op.
//
// Usage:
//
//	merged, cancel := contextx.Merge(requestCtx, backgroundCtx)
//	defer cancel()
//	// merged.Done() fires when requestCtx is cancelled OR cancel() is called
func Merge(parent1, parent2 context.Context) (context.Context, context.CancelFunc) {
	if parent1 == nil {
		parent1 = context.Background()
	}
	if parent2 == nil {
		parent2 = context.Background()
	}
	mc := &mergeCtx{
		parent1:  parent1,
		parent2:  parent2,
		done:     make(chan struct{}),
		cancelCh: make(chan struct{}),
	}
	select {
	case <-parent1.Done():
		_ = mc.finish(parent1.Err())
	case <-parent2.Done():
		_ = mc.finish(parent2.Err())
	default:
		go mc.wait()
	}
	return mc, mc.cancel
}

func (mc *mergeCtx) finish(err error) error {
	mc.doneOnce.Do(func() {
		mc.doneErr = err
		mc.doneMark.Store(true)
		close(mc.done)
	})
	return mc.doneErr
}

func (mc *mergeCtx) wait() {
	var err error
	select {
	case <-mc.parent1.Done():
		err = mc.parent1.Err()
	case <-mc.parent2.Done():
		err = mc.parent2.Err()
	case <-mc.cancelCh:
		err = context.Canceled
	}
	_ = mc.finish(err)
}

func (mc *mergeCtx) cancel() {
	mc.cancelOnce.Do(func() {
		close(mc.cancelCh)
	})
}

// Done implements context.Context.
func (mc *mergeCtx) Done() <-chan struct{} {
	return mc.done
}

// Err implements context.Context.
func (mc *mergeCtx) Err() error {
	if mc.doneMark.Load() {
		return mc.doneErr
	}
	var err error
	select {
	case <-mc.parent1.Done():
		err = mc.parent1.Err()
	case <-mc.parent2.Done():
		err = mc.parent2.Err()
	case <-mc.cancelCh:
		err = context.Canceled
	default:
		return nil
	}
	return mc.finish(err)
}

// Deadline implements context.Context. Returns the earlier of the two parents'
// deadlines.
func (mc *mergeCtx) Deadline() (time.Time, bool) {
	d1, ok1 := mc.parent1.Deadline()
	d2, ok2 := mc.parent2.Deadline()
	switch {
	case !ok1:
		return d2, ok2
	case !ok2:
		return d1, ok1
	case d1.Before(d2):
		return d1, true
	default:
		return d2, true
	}
}

// Value implements context.Context. parent1 takes precedence over parent2.
func (mc *mergeCtx) Value(key any) any {
	if v := mc.parent1.Value(key); v != nil {
		return v
	}
	return mc.parent2.Value(key)
}
