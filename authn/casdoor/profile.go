package casdoor

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/authn"
)

var _ authn.ProfileService = (*Client)(nil)

func (c *Client) GetIdentityProfile(ctx context.Context, req authn.IdentityProfileRequest) (authn.IdentityProfile, error) {
	principal, err := c.resolveProfilePrincipal(ctx, req)
	if err != nil {
		return authn.IdentityProfile{}, err
	}

	orgID := firstNonEmpty(req.OrgID, principal.OrgID, c.cfg.OrganizationName)
	appID := firstNonEmpty(req.AppID, principal.AppID, c.cfg.ApplicationName)
	principal.OrgID = firstNonEmpty(principal.OrgID, orgID)
	principal.AppID = firstNonEmpty(principal.AppID, appID)

	includeUser, includeGroups, includeApp := profileIncludes(req)
	profile := authn.IdentityProfile{
		Principal: principal.Normalize(),
		Attributes: authn.AttributeSet{
			"provider": ProviderName,
			"org_id":   orgID,
			"app_id":   appID,
		},
	}

	if includeUser {
		user, err := c.loadProfileUser(ctx, principal, orgID)
		if err != nil {
			if !req.AllowPartial {
				return authn.IdentityProfile{}, err
			}
			profile.Warnings = append(profile.Warnings, err.Error())
		} else {
			profile.User = user
			if profile.User.ID != "" || profile.User.Username != "" {
				profile.Attributes["user_loaded"] = true
			}
		}
	}

	if includeGroups {
		groups, warnings, err := c.loadProfileGroups(ctx, principal, profile.User, orgID)
		if err != nil {
			if !req.AllowPartial {
				return authn.IdentityProfile{}, err
			}
			profile.Warnings = append(profile.Warnings, err.Error())
		} else {
			profile.Groups = groups
		}
		profile.Warnings = append(profile.Warnings, warnings...)
	}

	if includeApp {
		app, err := c.GetApplication(ctx, orgID, appID)
		if err != nil {
			if !req.AllowPartial {
				return authn.IdentityProfile{}, err
			}
			profile.Warnings = append(profile.Warnings, fmt.Sprintf("load current application %q failed: %v", appID, err))
		} else {
			profile.CurrentApplication = app
			if app.ID != "" || app.Name != "" {
				profile.Attributes["current_application_loaded"] = true
			}
		}
	}

	return profile, nil
}

func (c *Client) resolveProfilePrincipal(ctx context.Context, req authn.IdentityProfileRequest) (authn.Principal, error) {
	if p := req.Principal.Normalize(); p.IsAuthenticated() {
		return p, nil
	}
	if strings.TrimSpace(req.Token) != "" {
		return c.VerifyToken(ctx, authn.VerifyTokenRequest{Token: req.Token, TokenType: "access_token", OrgID: req.OrgID, AppID: req.AppID})
	}
	if strings.TrimSpace(req.Credential.Token) != "" {
		return c.Authenticate(ctx, req.Credential)
	}
	return authn.Principal{}, authn.ErrMissingCredential("principal, token or credential is required to load identity profile")
}

func profileIncludes(req authn.IdentityProfileRequest) (includeUser, includeGroups, includeApp bool) {
	// Default to the useful post-login profile when the caller did not specify a
	// narrower projection.
	if !req.IncludeUser && !req.IncludeGroups && !req.IncludeCurrentApplication {
		return true, true, true
	}
	return req.IncludeUser, req.IncludeGroups, req.IncludeCurrentApplication
}

func (c *Client) loadProfileUser(ctx context.Context, principal authn.Principal, orgID string) (authn.User, error) {
	userID := firstNonEmpty(principal.Username, stringAttr(principal.Attributes, "casdoor_name"), principal.SubjectID)
	if userID == "" {
		return authn.User{}, authn.ErrUnauthenticated("cannot load profile user without username or subject id")
	}
	user, err := c.GetUser(ctx, orgID, userID)
	if err != nil {
		return authn.User{}, fmt.Errorf("load user %q failed: %w", userID, err)
	}
	return user, nil
}

func (c *Client) loadProfileGroups(ctx context.Context, principal authn.Principal, user authn.User, orgID string) ([]authn.Group, []string, error) {
	refs := mergeStrings(principal.Groups, user.Groups)
	if len(refs) == 0 {
		userID := firstNonEmpty(user.Username, principal.Username, principal.SubjectID)
		if userID == "" {
			return nil, nil, nil
		}
		groups, err := c.ListGroups(ctx, authn.GroupFilter{OrgID: orgID, UserID: userID})
		if err != nil {
			return nil, nil, fmt.Errorf("list groups for user %q failed: %w", userID, err)
		}
		return groups, nil, nil
	}

	groups := make([]authn.Group, 0, len(refs))
	warnings := make([]string, 0)
	for _, ref := range refs {
		group, err := c.GetGroup(ctx, orgID, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("load group %q failed: %v", ref, err))
			groups = append(groups, authn.Group{ID: ref, Name: ref, OrgID: orgID})
			continue
		}
		groups = append(groups, group)
	}
	return groups, warnings, nil
}

func mergeStrings(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, values := range groups {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func stringAttr(attrs authn.AttributeSet, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key].(string); ok {
		return v
	}
	return ""
}
