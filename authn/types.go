// Package authn defines Kernel's provider-neutral authentication contracts.
//
// authn owns the question "who is the caller?". It deliberately does not
// decide whether the caller can access a business resource; that belongs to
// authz. Casdoor, OIDC, JWT, mTLS and API-key implementations should all adapt
// into these interfaces.
package authn

import (
	"strings"
	"time"
)

// AttributeSet carries provider-specific claims or admin metadata. Values
// should be JSON-serializable when they are persisted or sent to remote systems.
type AttributeSet map[string]any

const (
	SubjectTypeUser      = "user"
	SubjectTypeService   = "service"
	SubjectTypeAgent     = "agent"
	SubjectTypeWorkflow  = "workflow"
	SubjectTypeWorkload  = "workload"
	SubjectTypeAnonymous = "anonymous"
)

const (
	AuthMethodOAuth    = "oauth"
	AuthMethodOIDC     = "oidc"
	AuthMethodJWT      = "jwt"
	AuthMethodAPIKey   = "api_key"
	AuthMethodMTLS     = "mtls"
	AuthMethodSession  = "session"
	AuthMethodDevToken = "dev_token"
	AuthMethodNone     = "none"
)

const (
	CredentialBearer = "bearer"
	CredentialBasic  = "basic"
	CredentialAPIKey = "api_key"
	CredentialMTLS   = "mtls"
	CredentialCode   = "authorization_code"
)

const (
	// Casdoor group types. A user can belong to one Physical group and multiple Virtual groups.
	GroupTypePhysical = "Physical"
	GroupTypeVirtual  = "Virtual"
)

// Principal is the normalized identity passed to business code.
//
// SubjectID must be stable inside Kernel. For Casdoor this should normally map
// from the Casdoor user's stable UUID when available, not only owner/name.
type Principal struct {
	SubjectID   string
	SubjectType string

	Provider   string
	ExternalID string
	Issuer     string
	Audience   []string

	TenantID  string
	OrgID     string
	AppID     string
	ProjectID string

	Username string
	Name     string
	Email    string
	Phone    string

	Roles  []string
	Groups []string
	Scopes []string

	// AuthMethod records how the subject authenticated. It is intentionally a
	// string instead of an enum type so adapters can preserve provider-specific
	// methods while still using the AuthMethod* constants above.
	AuthMethod string

	Attributes AttributeSet
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

func (p Principal) Normalize() Principal {
	p.SubjectID = strings.TrimSpace(p.SubjectID)
	p.SubjectType = strings.TrimSpace(p.SubjectType)
	if p.SubjectType == "" {
		p.SubjectType = SubjectTypeUser
	}
	p.Provider = strings.TrimSpace(p.Provider)
	p.ExternalID = strings.TrimSpace(p.ExternalID)
	p.TenantID = strings.TrimSpace(p.TenantID)
	p.OrgID = strings.TrimSpace(p.OrgID)
	p.AppID = strings.TrimSpace(p.AppID)
	p.ProjectID = strings.TrimSpace(p.ProjectID)
	p.Username = strings.TrimSpace(p.Username)
	p.Email = strings.TrimSpace(p.Email)
	return p
}

func (p Principal) IsAuthenticated() bool {
	p = p.Normalize()
	return p.SubjectID != "" && p.SubjectType != SubjectTypeAnonymous
}

func Anonymous() Principal {
	return Principal{SubjectType: SubjectTypeAnonymous, AuthMethod: AuthMethodNone}
}

func (p Principal) IsAnonymous() bool {
	return !p.IsAuthenticated()
}

func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (p Principal) HasGroup(group string) bool {
	for _, g := range p.Groups {
		if g == group {
			return true
		}
	}
	return false
}

func (p Principal) HasScope(scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (p Principal) Attribute(key string) any {
	if p.Attributes == nil {
		return nil
	}
	return p.Attributes[key]
}

func (p Principal) Expired(now time.Time) bool {
	return !p.ExpiresAt.IsZero() && !now.Before(p.ExpiresAt)
}

// Credential is authentication input extracted by HTTP/gRPC middleware.
type Credential struct {
	Scheme   string
	Token    string
	Metadata map[string]string
}

// TokenSet is the result of OAuth/OIDC token operations.
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	Scope        string
	ExpiresAt    time.Time
	Raw          AttributeSet
}

// LoginURLRequest asks an identity provider for a hosted login URL.
type LoginURLRequest struct {
	RedirectURI string
	State       string
	Scope       string
	OrgID       string
	AppID       string
	Metadata    map[string]string
}

// LoginURL is the safe redirect target returned by LoginService.
type LoginURL struct {
	URL         string
	RedirectURI string
	State       string
	Scope       string
	Provider    string
	OrgID       string
	AppID       string
}

// LogoutURLRequest asks an identity provider for a hosted logout URL.
type LogoutURLRequest struct {
	PostLogoutRedirectURI string
	IDTokenHint           string
	State                 string
	OrgID                 string
	AppID                 string
	Metadata              map[string]string
}

// LogoutURL is the safe redirect target returned by LogoutService.
type LogoutURL struct {
	URL                   string
	PostLogoutRedirectURI string
	State                 string
	Provider              string
	OrgID                 string
	AppID                 string
}

// CallbackRequest is the provider-neutral OAuth/OIDC callback payload.
type CallbackRequest struct {
	Code          string
	State         string
	ExpectedState string
	RedirectURI   string
	OrgID         string
	AppID         string
	Metadata      map[string]string
}

// CallbackResult is the result of a successful browser callback flow.
type CallbackResult struct {
	Tokens    TokenSet
	Principal Principal
	State     string
	Provider  string
}

// AuthCodeExchangeRequest is the provider-neutral shape for OAuth code exchange.
type AuthCodeExchangeRequest struct {
	Code         string
	State        string
	RedirectURI  string
	CodeVerifier string
	OrgID        string
	AppID        string
	Metadata     map[string]string
}

type RefreshTokenRequest struct {
	RefreshToken string
	Scope        string
	OrgID        string
	AppID        string
	Metadata     map[string]string
}

type VerifyTokenRequest struct {
	Token     string
	TokenType string
	Issuer    string
	Audience  []string
	OrgID     string
	AppID     string
	Leeway    time.Duration
	Metadata  map[string]string
}

type RevokeTokenRequest struct {
	Token     string
	TokenType string
	OrgID     string
	AppID     string
	Metadata  map[string]string
}

// User mirrors identity-provider user data without importing a provider SDK.
type User struct {
	ID          string
	ExternalID  string
	Provider    string
	OrgID       string
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Roles       []string
	Groups      []string
	Enabled     bool
	Attributes  AttributeSet
}

type UserFilter struct {
	ID         string
	ExternalID string
	OrgID      string
	Username   string
	Email      string
	Phone      string
	GroupID    string
	Role       string
	Limit      int
	Offset     int
}

// Organization is Kernel's identity-side organization projection. Kernel IAM DB
// can store richer business org metadata; this type is for authn provider sync.
type Organization struct {
	ID          string
	ExternalID  string
	Name        string
	DisplayName string
	OwnerID     string
	ParentID    string
	Tags        []string
	Enabled     bool
	Attributes  AttributeSet
}

type CreateOrganizationRequest struct {
	Organization   Organization
	IdempotencyKey string
	Metadata       map[string]string
}

type UpdateOrganizationRequest struct {
	Organization Organization
	Metadata     map[string]string
}

type DeleteOrganizationRequest struct {
	OrgID    string
	Hard     bool
	Metadata map[string]string
}

// Application represents a login client/application in the identity provider.
type Application struct {
	ID             string
	ExternalID     string
	OrgID          string
	Name           string
	DisplayName    string
	ClientID       string
	ClientSecret   string
	RedirectURIs   []string
	GrantTypes     []string
	Scopes         []string
	Providers      []string
	EnablePassword bool
	EnableSignup   bool
	Attributes     AttributeSet
}

type CreateApplicationRequest struct {
	Application    Application
	IdempotencyKey string
	Metadata       map[string]string
}

type UpdateApplicationRequest struct {
	Application Application
	Metadata    map[string]string
}

type DeleteApplicationRequest struct {
	AppID    string
	OrgID    string
	Hard     bool
	Metadata map[string]string
}

// Group models Casdoor's organization tree/groups without coupling Kernel
// to Casdoor names. Group.Type can be physical, virtual, team, etc.
type Group struct {
	ID          string
	ExternalID  string
	OrgID       string
	ParentID    string
	Name        string
	DisplayName string
	Type        string
	Path        string
	Users       []string
	Attributes  AttributeSet
}

type GroupFilter struct {
	OrgID    string
	ParentID string
	Type     string
	UserID   string
	Limit    int
	Offset   int
}

type CreateGroupRequest struct {
	Group          Group
	IdempotencyKey string
	Metadata       map[string]string
}

type UpdateGroupRequest struct {
	Group    Group
	Metadata map[string]string
}

type DeleteGroupRequest struct {
	OrgID     string
	GroupID   string
	Recursive bool
	Metadata  map[string]string
}

type AssignUserToGroupRequest struct {
	OrgID    string
	GroupID  string
	UserID   string
	Metadata map[string]string
}
