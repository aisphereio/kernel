package authz

import "strings"

func ValidateCheckRequest(req CheckRequest) error {
	if req.Subject.IsZero() {
		return ErrInvalidRequest("subject type and id are required")
	}
	if req.Resource.IsZero() {
		return ErrInvalidRequest("resource type and id are required")
	}
	if strings.TrimSpace(req.Permission) == "" {
		return ErrInvalidRequest("permission is required")
	}
	return nil
}

func ValidateRelationship(r Relationship) error {
	if r.Resource.IsZero() {
		return ErrInvalidRequest("relationship resource type and id are required")
	}
	if strings.TrimSpace(r.Relation) == "" {
		return ErrInvalidRequest("relationship relation is required")
	}
	if r.Subject.IsZero() {
		return ErrInvalidRequest("relationship subject type and id are required")
	}
	return nil
}
