package resourcex

import (
	"fmt"
	"strings"
)

func ValidateResourceType(typ ResourceType) error {
	if strings.TrimSpace(typ.Type) == "" {
		return fmt.Errorf("resource type is required")
	}
	if strings.TrimSpace(typ.Capability) == "" {
		return fmt.Errorf("resource type capability is required")
	}
	if strings.TrimSpace(typ.OwnerService) == "" {
		return fmt.Errorf("resource type owner service is required")
	}
	if strings.TrimSpace(typ.SpiceDBType) == "" {
		return fmt.Errorf("resource type spicedb type is required")
	}
	return nil
}

func ValidateResource(resource Resource) error {
	if resource.Ref.IsZero() {
		return fmt.Errorf("resource type and id are required")
	}
	if strings.TrimSpace(resource.OrgID) == "" {
		return fmt.Errorf("resource org id is required")
	}
	if strings.TrimSpace(resource.OwnerService) == "" {
		return fmt.Errorf("resource owner service is required")
	}
	if strings.TrimSpace(resource.OwnerResourceID) == "" {
		return fmt.Errorf("resource owner resource id is required")
	}
	return nil
}

func ValidateBinding(binding ResourceBinding) error {
	if binding.Source.IsZero() {
		return fmt.Errorf("binding source type and id are required")
	}
	if strings.TrimSpace(binding.Relation) == "" {
		return fmt.Errorf("binding relation is required")
	}
	if binding.Target.IsZero() {
		return fmt.Errorf("binding target type and id are required")
	}
	return nil
}

func ValidateRoleTemplate(tpl RoleTemplate) error {
	if strings.TrimSpace(tpl.ResourceType) == "" {
		return fmt.Errorf("role template resource type is required")
	}
	if strings.TrimSpace(tpl.RoleKey) == "" {
		return fmt.Errorf("role template role key is required")
	}
	if strings.TrimSpace(tpl.Relation) == "" {
		return fmt.Errorf("role template relation is required")
	}
	return nil
}

func ValidateGrant(grant Grant) error {
	if grant.Resource.IsZero() {
		return fmt.Errorf("grant resource type and id are required")
	}
	if strings.TrimSpace(grant.Relation) == "" {
		return fmt.Errorf("grant relation is required")
	}
	if grant.Subject.IsZero() {
		return fmt.Errorf("grant subject type and id are required")
	}
	return nil
}
