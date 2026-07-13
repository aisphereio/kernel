package taskx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrRuntimeUnavailable = errors.New("taskx: runtime unavailable")
	ErrHandlerExists      = errors.New("taskx: handler already registered")
)

// Runtime is the provider-neutral contract for a durable task runtime.
//
// A Runtime owns the schedule definition and dispatches trigger events back to
// one application replica. Implementations are expected to provide at-least-once
// delivery, therefore handlers must remain idempotent.
type Runtime interface {
	Schedule(context.Context, ManagedJob) error
	Get(context.Context, string) (ManagedJob, error)
	Delete(context.Context, string) error
	RegisterHandler(string, EventHandler) error
}

// ManagedJob is a durable job definition stored by an external task runtime.
// Schedule uses Kernel's canonical scheduler expression format:
//
//   - @every 5m
//   - @hourly, @daily, @weekly, @monthly, @yearly
//   - six-field cron: seconds minutes hours day-of-month month day-of-week
//
// DueTime and TTL accept RFC3339 timestamps or Go duration strings. At least
// one of Schedule or DueTime must be set.
type ManagedJob struct {
	Name          string
	Schedule      string
	DueTime       string
	Repeats       *uint32
	TTL           string
	Data          []byte
	DataTypeURL   string
	Overwrite     bool
	FailurePolicy *DeliveryFailurePolicy
}

func (j ManagedJob) Validate() error {
	if strings.TrimSpace(j.Name) == "" {
		return fmt.Errorf("%w: managed job name is required", ErrInvalidJob)
	}
	if strings.TrimSpace(j.Schedule) == "" && strings.TrimSpace(j.DueTime) == "" {
		return fmt.Errorf("%w: schedule or due time is required for %q", ErrInvalidJob, j.Name)
	}
	if j.FailurePolicy != nil {
		if err := j.FailurePolicy.Validate(); err != nil {
			return fmt.Errorf("%w: failure policy for %q: %v", ErrInvalidJob, j.Name, err)
		}
	}
	return nil
}

// DeliveryFailureMode controls what the runtime does when the application
// callback returns an error.
type DeliveryFailureMode string

const (
	DeliveryFailureConstant DeliveryFailureMode = "constant"
	DeliveryFailureDrop     DeliveryFailureMode = "drop"
)

// DeliveryFailurePolicy is deliberately smaller than the local RetryPolicy.
// It governs delivery from the runtime to the application, not retries inside
// a business handler.
type DeliveryFailurePolicy struct {
	Mode       DeliveryFailureMode
	MaxRetries *uint32
	Interval   time.Duration
}

func (p DeliveryFailurePolicy) Validate() error {
	switch p.Mode {
	case DeliveryFailureConstant:
		if p.Interval < 0 {
			return errors.New("interval must be non-negative")
		}
		return nil
	case DeliveryFailureDrop:
		if p.MaxRetries != nil || p.Interval != 0 {
			return errors.New("drop policy cannot define retries or interval")
		}
		return nil
	default:
		return fmt.Errorf("unsupported mode %q", p.Mode)
	}
}

// TriggerEvent is delivered by a Runtime when a managed job becomes due.
type TriggerEvent struct {
	Name string
	Data []byte
}

// EventHandler handles one durable runtime trigger.
type EventHandler func(context.Context, TriggerEvent) error
