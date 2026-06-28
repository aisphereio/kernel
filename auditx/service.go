package auditx

import (
	"context"
	"time"
)

// Recorder writes durable audit records. Implementations must be safe for
// concurrent use. Hot-path code should depend on this interface only.
type Recorder interface {
	Record(ctx context.Context, record Record) error
}

type Queryer interface {
	Query(ctx context.Context, filter QueryFilter) ([]Record, error)
}

type Store interface {
	Recorder
	Queryer
}

type QueryFilter struct {
	ActorID      string
	ActorType    string
	ResourceType string
	ResourceID   string
	Action       string
	Result       string
	TenantID     string
	OrgID        string
	ProjectID    string
	From         time.Time
	To           time.Time
	Limit        int
}

func Noop() Recorder { return noopRecorder{} }

// NoopStore returns a Store implementation that drops all audit records and
// always returns an empty query result. Use it when callers require the full
// Store surface but audit persistence is intentionally disabled.
func NoopStore() Store { return noopStore{} }

type noopRecorder struct{}

func (noopRecorder) Record(context.Context, Record) error { return nil }

type noopStore struct{}

func (noopStore) Record(context.Context, Record) error { return nil }

func (noopStore) Query(context.Context, QueryFilter) ([]Record, error) { return nil, nil }
