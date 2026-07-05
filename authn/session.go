package authn

import (
	"context"
	"time"
)

const (
	SessionStatusActive  = "active"
	SessionStatusExpired = "expired"
	SessionStatusRevoked = "revoked"
)

// Session is Kernel's provider-neutral view of an identity-provider session.
//
// Casdoor-first deployments must treat Casdoor as the source of truth for
// sessions. Kernel does not create, validate, refresh or persist sessions. This
// type is only used for admin/user-facing read models such as "who is online",
// "which users have active sessions", or "show my logged-in devices".
type Session struct {
	ID                string
	Provider          string
	ProviderSessionID string

	OrgID       string
	Application string

	SubjectID   string
	SubjectType string
	Username    string
	DisplayName string
	Email       string

	Status    string
	UserAgent string
	IP        string

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time

	Attributes AttributeSet
}

func (s Session) IsActive(now time.Time) bool {
	if s.Status != "" && s.Status != SessionStatusActive {
		return false
	}
	if !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt) {
		return false
	}
	return true
}

type SessionFilter struct {
	SessionID         string
	ProviderSessionID string
	OrgID             string
	Application       string
	SubjectID         string
	Username          string
	Email             string
	Status            string
	OnlineOnly        bool
	Limit             int
	Offset            int
}

type SessionSummary struct {
	TotalSessions  int
	ActiveSessions int
	OnlineUsers    int
	ByApplication  map[string]int
}

// SessionDirectory reads identity-provider sessions for display and operational
// visibility. It deliberately does not create or validate sessions; login,
// logout, refresh-token and browser session lifecycle remain owned by Casdoor or
// another external identity provider.
type SessionDirectory interface {
	GetSession(ctx context.Context, orgID, sessionID string) (Session, error)
	ListSessions(ctx context.Context, filter SessionFilter) ([]Session, error)
	SummarizeSessions(ctx context.Context, filter SessionFilter) (SessionSummary, error)
}
