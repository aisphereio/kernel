package dtmx

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Manager is the business-facing distributed transaction interface.
//
// Business modules should depend on Manager, Saga, and SagaStep only. They
// should not import github.com/dtm-labs/client directly.
type Manager interface {
	Enabled() bool
	DriverName() string
	NewGID(ctx context.Context) (string, error)
	SubmitSaga(ctx context.Context, saga Saga) (TransactionResult, error)
	BranchURL(path string) string
	Close() error
}

type TransactionResult struct {
	GID       string
	Protocol  string
	Pattern   string
	Submitted bool
	StartedAt time.Time
	Elapsed   time.Duration
}

type Saga struct {
	GID   string
	Name  string
	Steps []SagaStep

	// Options override Config defaults for a single Saga.
	Options SagaOptions
}

type SagaOptions struct {
	WaitResult    *bool
	Timeout       time.Duration
	BranchHeaders map[string]string
}

type SagaStep struct {
	Name       string
	Action     string
	Compensate string
	Payload    any
}

func NewSaga(gid, name string, opts ...SagaOption) Saga {
	s := Saga{GID: gid, Name: name}
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}
	return s
}

type SagaOption func(*Saga)

func WithSagaWaitResult(wait bool) SagaOption {
	return func(s *Saga) { s.Options.WaitResult = &wait }
}

func WithSagaTimeout(timeout time.Duration) SagaOption {
	return func(s *Saga) { s.Options.Timeout = timeout }
}

// WithSagaBranchHeader adds a header that DTM forwards to every branch
// action/compensate request for this Saga. Prefer Config.BranchSecret for the
// common shared-secret case; use this for per-transaction metadata only.
func WithSagaBranchHeader(key, value string) SagaOption {
	return func(s *Saga) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if s.Options.BranchHeaders == nil {
			s.Options.BranchHeaders = make(map[string]string)
		}
		s.Options.BranchHeaders[key] = value
	}
}

// WithSagaBranchHeaders adds headers that DTM forwards to every branch
// action/compensate request for this Saga.
func WithSagaBranchHeaders(headers map[string]string) SagaOption {
	return func(s *Saga) {
		if len(headers) == 0 {
			return
		}
		if s.Options.BranchHeaders == nil {
			s.Options.BranchHeaders = make(map[string]string, len(headers))
		}
		for k, v := range headers {
			k = strings.TrimSpace(k)
			if k != "" {
				s.Options.BranchHeaders[k] = v
			}
		}
	}
}

func (s Saga) Add(step SagaStep) Saga {
	s.Steps = append(s.Steps, step)
	return s
}

func (s Saga) AddHTTP(name, action, compensate string, payload any) Saga {
	return s.Add(SagaStep{Name: name, Action: action, Compensate: compensate, Payload: payload})
}

func (s Saga) Validate() error {
	if strings.TrimSpace(s.GID) == "" {
		return ErrGIDRequired
	}
	if len(s.Steps) == 0 {
		return ErrNoSteps
	}
	for i, step := range s.Steps {
		if strings.TrimSpace(step.Action) == "" || strings.TrimSpace(step.Compensate) == "" {
			return fmt.Errorf("%w: step[%d] action and compensate are required", ErrInvalidBranch, i)
		}
		if !isHTTPURL(step.Action) || !isHTTPURL(step.Compensate) {
			return fmt.Errorf("%w: step[%d] action and compensate must be absolute http(s) urls", ErrInvalidBranch, i)
		}
	}
	return nil
}

func isHTTPURL(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")
}
