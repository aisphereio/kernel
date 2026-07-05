package authn

import (
	"strings"
)

const (
	TrustedHeaderVerified    = "X-Aisphere-Auth-Verified"
	TrustedHeaderSubject     = "X-Aisphere-Subject"
	TrustedHeaderSubjectType = "X-Aisphere-Subject-Type"
	TrustedHeaderProvider    = "X-Aisphere-Provider"
	TrustedHeaderExternalID  = "X-Aisphere-External-ID"
	TrustedHeaderIssuer      = "X-Aisphere-Issuer"
	TrustedHeaderAudience    = "X-Aisphere-Audience"
	TrustedHeaderOwner       = "X-Aisphere-Owner"
	TrustedHeaderOrgID       = "X-Aisphere-Org-ID"
	TrustedHeaderAppID       = "X-Aisphere-App-ID"
	TrustedHeaderUsername    = "X-Aisphere-Username"
	TrustedHeaderName        = "X-Aisphere-Name"
	TrustedHeaderEmail       = "X-Aisphere-Email"
	TrustedHeaderGroups      = "X-Aisphere-Groups"
	TrustedHeaderRoles       = "X-Aisphere-Roles"
	TrustedHeaderScopes      = "X-Aisphere-Scopes"
)

var trustedHeaderNames = []string{
	TrustedHeaderVerified,
	TrustedHeaderSubject,
	TrustedHeaderSubjectType,
	TrustedHeaderProvider,
	TrustedHeaderExternalID,
	TrustedHeaderIssuer,
	TrustedHeaderAudience,
	TrustedHeaderOwner,
	TrustedHeaderOrgID,
	TrustedHeaderAppID,
	TrustedHeaderUsername,
	TrustedHeaderName,
	TrustedHeaderEmail,
	TrustedHeaderGroups,
	TrustedHeaderRoles,
	TrustedHeaderScopes,
}

// TrustedHeaderNames returns all identity headers controlled by Gateway. Any
// inbound client-supplied values for these headers must be stripped before a
// verified principal is injected.
func TrustedHeaderNames() []string {
	return append([]string(nil), trustedHeaderNames...)
}

// StripTrustedHeaders removes identity headers that must only be set by a
// trusted Gateway after token verification. It intentionally does not remove
// the internal-service-token header; use StripGatewayControlledHeaders at the
// edge before injecting both trusted identity and internal-call headers.
func StripTrustedHeaders(headers map[string]string) {
	if headers == nil {
		return
	}
	for existing := range headers {
		for _, trusted := range trustedHeaderNames {
			if strings.EqualFold(existing, trusted) {
				delete(headers, existing)
				break
			}
		}
	}
}

// InjectTrustedHeaders writes a normalized Principal to Gateway-controlled
// headers for downstream services configured in gateway_trusted mode.
func InjectTrustedHeaders(headers map[string]string, p Principal) {
	if headers == nil {
		return
	}
	p = p.Normalize()
	if !p.IsAuthenticated() {
		return
	}
	headers[TrustedHeaderVerified] = "true"
	headers[TrustedHeaderSubject] = p.SubjectID
	headers[TrustedHeaderSubjectType] = p.SubjectType
	headers[TrustedHeaderProvider] = p.Provider
	headers[TrustedHeaderExternalID] = p.ExternalID
	headers[TrustedHeaderIssuer] = p.Issuer
	headers[TrustedHeaderAudience] = strings.Join(p.Audience, ",")
	headers[TrustedHeaderOwner] = firstNonEmpty(p.OrgID, p.TenantID)
	headers[TrustedHeaderOrgID] = p.OrgID
	headers[TrustedHeaderAppID] = p.AppID
	headers[TrustedHeaderUsername] = p.Username
	headers[TrustedHeaderName] = p.Name
	headers[TrustedHeaderEmail] = p.Email
	headers[TrustedHeaderGroups] = strings.Join(p.Groups, ",")
	headers[TrustedHeaderRoles] = strings.Join(p.Roles, ",")
	headers[TrustedHeaderScopes] = strings.Join(p.Scopes, ",")
}

// PrincipalFromTrustedHeaders reconstructs a Principal that was already
// verified by Gateway. It must only be used on traffic that cannot bypass the
// trusted Gateway boundary.
func PrincipalFromTrustedHeaders(headers map[string]string) (Principal, bool) {
	if !strings.EqualFold(headerValue(headers, TrustedHeaderVerified), "true") {
		return Principal{}, false
	}
	p := Principal{
		SubjectID:   headerValue(headers, TrustedHeaderSubject),
		SubjectType: headerValue(headers, TrustedHeaderSubjectType),
		Provider:    headerValue(headers, TrustedHeaderProvider),
		ExternalID:  headerValue(headers, TrustedHeaderExternalID),
		Issuer:      headerValue(headers, TrustedHeaderIssuer),
		Audience:    splitCSV(headerValue(headers, TrustedHeaderAudience)),
		TenantID:    headerValue(headers, TrustedHeaderOwner),
		OrgID:       headerValue(headers, TrustedHeaderOrgID),
		AppID:       headerValue(headers, TrustedHeaderAppID),
		Username:    headerValue(headers, TrustedHeaderUsername),
		Name:        headerValue(headers, TrustedHeaderName),
		Email:       headerValue(headers, TrustedHeaderEmail),
		Groups:      splitCSV(headerValue(headers, TrustedHeaderGroups)),
		Roles:       splitCSV(headerValue(headers, TrustedHeaderRoles)),
		Scopes:      splitCSV(headerValue(headers, TrustedHeaderScopes)),
		AuthMethod:  AuthMethodJWT,
	}.Normalize()
	return p, p.IsAuthenticated()
}

func headerValue(headers map[string]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// StripGatewayControlledHeaders removes every header that can only be set by
// Gateway after it has verified the external Casdoor/OIDC JWT. Call this at the
// edge before injecting trusted Principal headers and the internal service token.
func StripGatewayControlledHeaders(headers map[string]string, internalTokenHeader string) {
	StripTrustedHeaders(headers)
	StripInternalServiceToken(headers, internalTokenHeader)
}
