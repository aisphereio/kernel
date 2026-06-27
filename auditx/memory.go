package auditx

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	now     func() time.Time
	records []Record
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{now: time.Now} }

func (s *MemoryStore) Record(ctx context.Context, record Record) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := Validate(record); err != nil {
		return err
	}
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	record = record.WithTime(now())
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *MemoryStore) Query(ctx context.Context, filter QueryFilter) ([]Record, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		if recordMatches(record, filter) {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[len(out)-filter.Limit:]
	}
	return out, nil
}

func recordMatches(r Record, f QueryFilter) bool {
	if f.ActorID != "" && r.Actor.SubjectID != f.ActorID {
		return false
	}
	if f.ActorType != "" && r.Actor.SubjectType != f.ActorType {
		return false
	}
	if f.ResourceType != "" && r.Resource.Type != f.ResourceType {
		return false
	}
	if f.ResourceID != "" && r.Resource.ID != f.ResourceID {
		return false
	}
	if f.Action != "" && r.Action != f.Action {
		return false
	}
	if f.Result != "" && r.Result != f.Result {
		return false
	}
	if f.TenantID != "" && r.Actor.TenantID != f.TenantID && r.Resource.TenantID != f.TenantID {
		return false
	}
	if f.OrgID != "" && r.Actor.OrgID != f.OrgID && r.Resource.OrgID != f.OrgID {
		return false
	}
	if f.ProjectID != "" && r.Actor.ProjectID != f.ProjectID && r.Resource.ProjectID != f.ProjectID {
		return false
	}
	if !f.From.IsZero() && r.Time.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && !r.Time.Before(f.To) {
		return false
	}
	return true
}
