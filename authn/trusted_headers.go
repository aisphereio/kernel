package authn

import (
	"strings"
	"unicode"
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
	TrustedHeaderProjectID   = "X-Aisphere-Project-ID"
	TrustedHeaderUsername    = "X-Aisphere-Username"
	TrustedHeaderName        = "X-Aisphere-Name"
	TrustedHeaderEmail       = "X-Aisphere-Email"
	TrustedHeaderPhone       = "X-Aisphere-Phone"
	TrustedHeaderGroups      = "X-Aisphere-Groups"
	TrustedHeaderRoles       = "X-Aisphere-Roles"
	TrustedHeaderScopes      = "X-Aisphere-Scopes"
)

const (
	// GatewayClaimHeaderExternalSub is the normalized Envoy Gateway claimToHeaders
	// output for the verified OIDC subject. This is the primary identity key for
	// OIDC-only Gateway deployments.
	GatewayClaimHeaderExternalSub         = "X-Aisphere-External-Sub"
	GatewayClaimHeaderExternalIssuer      = "X-Aisphere-External-Issuer"
	GatewayClaimHeaderExternalEmail       = "X-Aisphere-External-Email"
	GatewayClaimHeaderExternalName        = "X-Aisphere-External-Name"
	GatewayClaimHeaderExternalDisplayName = "X-Aisphere-External-Display-Name"
	GatewayClaimHeaderExternalUsername    = "X-Aisphere-External-Username"
	GatewayClaimHeaderExternalPhone       = "X-Aisphere-External-Phone"
	GatewayClaimHeaderExternalOwner       = "X-Aisphere-External-Owner"
	GatewayClaimHeaderExternalScope       = "X-Aisphere-External-Scope"
	GatewayClaimHeaderExternalID          = TrustedHeaderExternalID

	// Optional internal projection headers. They are only trustworthy when the
	// edge Gateway strips client-supplied values before injecting verified claims.
	GatewayClaimHeaderPrincipal = "X-Aisphere-Principal"
	GatewayClaimHeaderUserID    = "X-Aisphere-User-ID"
	GatewayClaimHeaderOrgID     = TrustedHeaderOrgID
	GatewayClaimHeaderProjectID = TrustedHeaderProjectID
	GatewayClaimHeaderRoles     = TrustedHeaderRoles
	GatewayClaimHeaderGroups    = TrustedHeaderGroups
	GatewayClaimHeaderScopes    = TrustedHeaderScopes
	GatewayClaimHeaderProvider  = TrustedHeaderProvider
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
	TrustedHeaderProjectID,
	TrustedHeaderUsername,
	TrustedHeaderName,
	TrustedHeaderEmail,
	TrustedHeaderPhone,
	TrustedHeaderGroups,
	TrustedHeaderRoles,
	TrustedHeaderScopes,
	GatewayClaimHeaderExternalSub,
	GatewayClaimHeaderExternalIssuer,
	GatewayClaimHeaderExternalEmail,
	GatewayClaimHeaderExternalName,
	GatewayClaimHeaderExternalDisplayName,
	GatewayClaimHeaderExternalUsername,
	GatewayClaimHeaderExternalPhone,
	GatewayClaimHeaderExternalOwner,
	GatewayClaimHeaderExternalScope,
	GatewayClaimHeaderPrincipal,
	GatewayClaimHeaderUserID,
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
	externalID := firstNonEmpty(p.ExternalID, p.SubjectID)
	ownerID := firstNonEmpty(p.OrgID, p.TenantID)
	displayName := firstNonEmpty(p.Name, p.Username)

	headers[TrustedHeaderVerified] = "true"
	headers[TrustedHeaderSubject] = p.SubjectID
	headers[TrustedHeaderSubjectType] = p.SubjectType
	headers[TrustedHeaderProvider] = p.Provider
	headers[TrustedHeaderExternalID] = externalID
	headers[TrustedHeaderIssuer] = p.Issuer
	headers[TrustedHeaderAudience] = strings.Join(p.Audience, ",")
	headers[TrustedHeaderOwner] = ownerID
	headers[TrustedHeaderOrgID] = p.OrgID
	headers[TrustedHeaderAppID] = p.AppID
	headers[TrustedHeaderProjectID] = p.ProjectID
	headers[TrustedHeaderUsername] = p.Username
	headers[TrustedHeaderName] = p.Name
	headers[TrustedHeaderEmail] = p.Email
	headers[TrustedHeaderPhone] = p.Phone
	headers[TrustedHeaderGroups] = strings.Join(p.Groups, ",")
	headers[TrustedHeaderRoles] = strings.Join(p.Roles, ",")
	headers[TrustedHeaderScopes] = strings.Join(p.Scopes, ",")

	// Also write the Envoy claim-header names so downstream services that only
	// understand the OIDC-only Gateway contract can consume the same principal.
	headers[GatewayClaimHeaderExternalSub] = externalID
	headers[GatewayClaimHeaderExternalIssuer] = p.Issuer
	headers[GatewayClaimHeaderExternalEmail] = p.Email
	headers[GatewayClaimHeaderExternalName] = firstNonEmpty(p.Username, p.Name)
	headers[GatewayClaimHeaderExternalDisplayName] = displayName
	headers[GatewayClaimHeaderExternalUsername] = p.Username
	headers[GatewayClaimHeaderExternalPhone] = p.Phone
	headers[GatewayClaimHeaderExternalOwner] = ownerID
	headers[GatewayClaimHeaderExternalScope] = strings.Join(p.Scopes, " ")
	headers[GatewayClaimHeaderPrincipal] = p.SubjectID
	headers[GatewayClaimHeaderUserID] = p.SubjectID
}

// PrincipalFromTrustedHeaders reconstructs a Principal that was already
// verified by Gateway. It must only be used on traffic that cannot bypass the
// trusted Gateway boundary.
func PrincipalFromTrustedHeaders(headers map[string]string) (Principal, bool) {
	if strings.EqualFold(headerValue(headers, TrustedHeaderVerified), "true") {
		return principalFromExplicitTrustedHeaders(headers)
	}
	return principalFromGatewayClaimHeaders(headers)
}

func principalFromExplicitTrustedHeaders(headers map[string]string) (Principal, bool) {
	ownerID := firstNonEmpty(headerValue(headers, TrustedHeaderOwner), headerValue(headers, TrustedHeaderOrgID))
	p := Principal{
		SubjectID:   headerValue(headers, TrustedHeaderSubject),
		SubjectType: headerValue(headers, TrustedHeaderSubjectType),
		Provider:    headerValue(headers, TrustedHeaderProvider),
		ExternalID:  headerValue(headers, TrustedHeaderExternalID),
		Issuer:      headerValue(headers, TrustedHeaderIssuer),
		Audience:    splitCSV(headerValue(headers, TrustedHeaderAudience)),
		TenantID:    ownerID,
		OrgID:       firstNonEmpty(headerValue(headers, TrustedHeaderOrgID), ownerID),
		AppID:       headerValue(headers, TrustedHeaderAppID),
		ProjectID:   headerValue(headers, TrustedHeaderProjectID),
		Username:    headerValue(headers, TrustedHeaderUsername),
		Name:        headerValue(headers, TrustedHeaderName),
		Email:       headerValue(headers, TrustedHeaderEmail),
		Phone:       headerValue(headers, TrustedHeaderPhone),
		Groups:      splitCSV(headerValue(headers, TrustedHeaderGroups)),
		Roles:       splitCSV(headerValue(headers, TrustedHeaderRoles)),
		Scopes:      splitCSV(headerValue(headers, TrustedHeaderScopes)),
		AuthMethod:  AuthMethodJWT,
	}.Normalize()
	if p.ExternalID == "" {
		p.ExternalID = p.SubjectID
	}
	return p, p.IsAuthenticated()
}

func principalFromGatewayClaimHeaders(headers map[string]string) (Principal, bool) {
	externalSub := headerValue(headers, GatewayClaimHeaderExternalSub)
	externalID := headerValue(headers, GatewayClaimHeaderExternalID)
	principalID := headerValue(headers, GatewayClaimHeaderPrincipal)
	userID := headerValue(headers, GatewayClaimHeaderUserID)
	subjectID := firstNonEmpty(principalID, userID, externalID, externalSub)
	if subjectID == "" {
		return Principal{}, false
	}

	externalUsername := headerValue(headers, GatewayClaimHeaderExternalUsername)
	externalName := headerValue(headers, GatewayClaimHeaderExternalName)
	externalDisplayName := headerValue(headers, GatewayClaimHeaderExternalDisplayName)
	externalOwner := headerValue(headers, GatewayClaimHeaderExternalOwner)
	ownerID := firstNonEmpty(headerValue(headers, GatewayClaimHeaderOrgID), externalOwner, headerValue(headers, TrustedHeaderOwner))
	scopes := firstNonEmpty(headerValue(headers, GatewayClaimHeaderScopes), headerValue(headers, GatewayClaimHeaderExternalScope))

	p := Principal{
		SubjectID:   subjectID,
		SubjectType: SubjectTypeUser,
		Provider:    firstNonEmpty(headerValue(headers, GatewayClaimHeaderProvider), headerValue(headers, TrustedHeaderProvider), "gateway"),
		ExternalID:  firstNonEmpty(externalID, externalSub, subjectID),
		Issuer:      headerValue(headers, GatewayClaimHeaderExternalIssuer),
		TenantID:    ownerID,
		OrgID:       ownerID,
		ProjectID:   headerValue(headers, GatewayClaimHeaderProjectID),
		Username:    firstNonEmpty(externalUsername, externalName),
		Name:        firstNonEmpty(externalDisplayName, externalName, externalUsername),
		Email:       headerValue(headers, GatewayClaimHeaderExternalEmail),
		Phone:       headerValue(headers, GatewayClaimHeaderExternalPhone),
		Groups:      splitCSV(headerValue(headers, GatewayClaimHeaderGroups)),
		Roles:       splitCSV(headerValue(headers, GatewayClaimHeaderRoles)),
		Scopes:      splitCSV(scopes),
		AuthMethod:  AuthMethodOIDC,
		Attributes: AttributeSet{
			"gateway.external_sub":        externalSub,
			"gateway.external_id":         externalID,
			"gateway.external_owner":      externalOwner,
			"gateway.external_name":       externalName,
			"gateway.external_displayName": externalDisplayName,
			"gateway.principal":           principalID,
			"gateway.user_id":             userID,
		},
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
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})
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
