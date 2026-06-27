package auditx

import "strings"

func Validate(record Record) error {
	if strings.TrimSpace(record.Action) == "" {
		return ErrInvalidRecord("audit action is required")
	}
	if strings.TrimSpace(record.Result) == "" {
		return ErrInvalidRecord("audit result is required")
	}
	if strings.TrimSpace(record.Actor.SubjectID) == "" {
		return ErrInvalidRecord("audit actor subject id is required")
	}
	if strings.TrimSpace(record.Resource.Type) == "" || strings.TrimSpace(record.Resource.ID) == "" {
		return ErrInvalidRecord("audit resource type and id are required")
	}
	return nil
}
