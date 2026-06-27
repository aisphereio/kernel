package authn

import "strings"

func ValidateAuthCodeExchangeRequest(req AuthCodeExchangeRequest) error {
	if strings.TrimSpace(req.Code) == "" {
		return ErrInvalidTokenRequest("authorization code is required")
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return ErrInvalidTokenRequest("redirect uri is required")
	}
	return nil
}

func ValidateOrganization(org Organization) error {
	if strings.TrimSpace(org.ID) == "" && strings.TrimSpace(org.Name) == "" {
		return ErrInvalidTokenRequest("organization id or name is required")
	}
	return nil
}

func ValidateApplication(app Application) error {
	if strings.TrimSpace(app.OrgID) == "" {
		return ErrInvalidTokenRequest("application org id is required")
	}
	if strings.TrimSpace(app.ID) == "" && strings.TrimSpace(app.Name) == "" {
		return ErrInvalidTokenRequest("application id or name is required")
	}
	return nil
}

func ValidateGroup(group Group) error {
	if strings.TrimSpace(group.OrgID) == "" {
		return ErrInvalidTokenRequest("group org id is required")
	}
	if strings.TrimSpace(group.ID) == "" && strings.TrimSpace(group.Name) == "" {
		return ErrInvalidTokenRequest("group id or name is required")
	}
	return nil
}
