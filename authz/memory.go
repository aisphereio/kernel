package authz

import (
	"context"
	"sort"
	"sync"
)

// MemoryRelationshipStore is for unit tests and local demos only. It does not
// evaluate recursive permissions; production ReBAC should use SpiceDB/OpenFGA.
type MemoryRelationshipStore struct {
	mu            sync.RWMutex
	relationships map[string]Relationship
}

func NewMemoryRelationshipStore() *MemoryRelationshipStore {
	return &MemoryRelationshipStore{relationships: make(map[string]Relationship)}
}

func (s *MemoryRelationshipStore) WriteRelationships(ctx context.Context, relationships ...Relationship) (WriteResult, error) {
	select {
	case <-ctx.Done():
		return WriteResult{}, ctx.Err()
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.relationships == nil {
		s.relationships = make(map[string]Relationship)
	}
	for _, relationship := range relationships {
		if err := ValidateRelationship(relationship); err != nil {
			return WriteResult{}, err
		}
		s.relationships[relationshipKey(relationship)] = relationship
	}
	return WriteResult{Written: len(relationships)}, nil
}

func (s *MemoryRelationshipStore) DeleteRelationships(ctx context.Context, filter RelationshipFilter) (WriteResult, error) {
	select {
	case <-ctx.Done():
		return WriteResult{}, ctx.Err()
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for key, relationship := range s.relationships {
		if relationshipMatches(relationship, filter) {
			delete(s.relationships, key)
			deleted++
		}
	}
	return WriteResult{Deleted: deleted}, nil
}

func (s *MemoryRelationshipStore) ReadRelationships(ctx context.Context, filter RelationshipFilter) ([]Relationship, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Relationship, 0, len(s.relationships))
	for _, relationship := range s.relationships {
		if relationshipMatches(relationship, filter) {
			out = append(out, relationship)
		}
	}
	sort.Slice(out, func(i, j int) bool { return relationshipKey(out[i]) < relationshipKey(out[j]) })
	return out, nil
}

// MemoryAuthorizer allows when resource#permission@subject exists exactly.
type MemoryAuthorizer struct{ Store RelationshipReader }

func NewMemoryAuthorizer(store RelationshipReader) MemoryAuthorizer {
	return MemoryAuthorizer{Store: store}
}

func (a MemoryAuthorizer) Check(ctx context.Context, req CheckRequest) (Decision, error) {
	if err := ValidateCheckRequest(req); err != nil {
		return Decision{}, err
	}
	if a.Store == nil {
		return Decision{Allowed: false, Reason: "relationship_store_not_configured"}, nil
	}
	relationships, err := a.Store.ReadRelationships(ctx, RelationshipFilter{
		ResourceType: req.Resource.Type,
		ResourceID:   req.Resource.ID,
		Relation:     req.Permission,
		SubjectType:  req.Subject.Type,
		SubjectID:    req.Subject.ID,
		SubjectRel:   req.Subject.Relation,
	})
	if err != nil {
		return Decision{}, err
	}
	if len(relationships) == 0 {
		return Decision{Allowed: false, Reason: "relationship_not_found"}, nil
	}
	return Decision{Allowed: true, Reason: "relationship_match"}, nil
}

func relationshipKey(r Relationship) string {
	return r.Resource.Type + ":" + r.Resource.ID + "#" + r.Relation + "@" + r.Subject.Type + ":" + r.Subject.ID + "#" + r.Subject.Relation
}

func relationshipMatches(r Relationship, f RelationshipFilter) bool {
	if f.ResourceType != "" && r.Resource.Type != f.ResourceType {
		return false
	}
	if f.ResourceID != "" && r.Resource.ID != f.ResourceID {
		return false
	}
	if f.Relation != "" && r.Relation != f.Relation {
		return false
	}
	if f.SubjectType != "" && r.Subject.Type != f.SubjectType {
		return false
	}
	if f.SubjectID != "" && r.Subject.ID != f.SubjectID {
		return false
	}
	if f.SubjectRel != "" && r.Subject.Relation != f.SubjectRel {
		return false
	}
	return true
}
