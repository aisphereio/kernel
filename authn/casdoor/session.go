package casdoor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

var _ authn.SessionDirectory = (*Client)(nil)

func (c *Client) GetSession(ctx context.Context, orgID, sessionID string) (authn.Session, error) {
	filter := authn.SessionFilter{OrgID: orgID, SessionID: sessionID}
	sessions, err := c.ListSessions(ctx, filter)
	if err != nil {
		return authn.Session{}, err
	}
	if len(sessions) == 0 {
		return authn.Session{}, authn.ErrUnauthenticated("casdoor session not found")
	}
	return sessions[0], nil
}

func (c *Client) ListSessions(ctx context.Context, filter authn.SessionFilter) ([]authn.Session, error) {
	_ = ctx
	sdkSessions, err := c.adminSDK().GetSessions()
	if err != nil {
		return nil, wrapBackend("casdoor list sessions failed", err)
	}
	out := make([]authn.Session, 0, len(sdkSessions))
	for _, session := range sdkSessions {
		if session == nil {
			continue
		}
		mapped := sessionsFromSDK(session)
		for _, item := range mapped {
			if matchesSessionFilter(item, filter) {
				out = append(out, item)
			}
		}
	}
	return paginateSessions(out, filter), nil
}

func (c *Client) SummarizeSessions(ctx context.Context, filter authn.SessionFilter) (authn.SessionSummary, error) {
	filter.Limit = 0
	filter.Offset = 0
	sessions, err := c.ListSessions(ctx, filter)
	if err != nil {
		return authn.SessionSummary{}, err
	}
	summary := authn.SessionSummary{ByApplication: map[string]int{}}
	users := map[string]struct{}{}
	now := time.Now()
	for _, session := range sessions {
		summary.TotalSessions++
		if session.IsActive(now) {
			summary.ActiveSessions++
			userKey := firstNonEmpty(session.SubjectID, session.Username, session.Email)
			if userKey != "" {
				users[userKey] = struct{}{}
			}
		}
		if session.Application != "" {
			summary.ByApplication[session.Application]++
		}
	}
	summary.OnlineUsers = len(users)
	return summary, nil
}

func sessionsFromSDK(session *casdoorsdk.Session) []authn.Session {
	if session == nil {
		return nil
	}
	createdAt := parseCasdoorTime(session.CreatedTime)
	sids := append([]string(nil), session.SessionId...)
	if len(sids) == 0 {
		sids = []string{""}
	}
	out := make([]authn.Session, 0, len(sids))
	for _, sid := range sids {
		id := sessionID(session.Owner, session.Name, session.Application, sid)
		out = append(out, authn.Session{
			ID:                id,
			Provider:          ProviderName,
			ProviderSessionID: sid,
			OrgID:             session.Owner,
			Application:       session.Application,
			SubjectID:         session.Name,
			SubjectType:       authn.SubjectTypeUser,
			Username:          session.Name,
			Status:            authn.SessionStatusActive,
			CreatedAt:         createdAt,
			Attributes: authn.AttributeSet{
				"owner":        session.Owner,
				"name":         session.Name,
				"application":  session.Application,
				"created_time": session.CreatedTime,
				"session_ids":  append([]string(nil), session.SessionId...),
			},
		})
	}
	return out
}

func matchesSessionFilter(session authn.Session, filter authn.SessionFilter) bool {
	if filter.SessionID != "" && session.ID != filter.SessionID && session.ProviderSessionID != filter.SessionID {
		return false
	}
	if filter.ProviderSessionID != "" && session.ProviderSessionID != filter.ProviderSessionID {
		return false
	}
	if filter.OrgID != "" && session.OrgID != filter.OrgID {
		return false
	}
	if filter.Application != "" && session.Application != filter.Application {
		return false
	}
	if filter.SubjectID != "" && session.SubjectID != filter.SubjectID {
		return false
	}
	if filter.Username != "" && session.Username != filter.Username {
		return false
	}
	if filter.Email != "" && !strings.EqualFold(session.Email, filter.Email) {
		return false
	}
	if filter.Status != "" && session.Status != filter.Status {
		return false
	}
	if filter.OnlineOnly && !session.IsActive(time.Now()) {
		return false
	}
	return true
}

func paginateSessions(sessions []authn.Session, filter authn.SessionFilter) []authn.Session {
	if filter.Offset > 0 {
		if filter.Offset >= len(sessions) {
			return []authn.Session{}
		}
		sessions = sessions[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(sessions) {
		sessions = sessions[:filter.Limit]
	}
	return sessions
}

func sessionID(owner, name, application, providerSessionID string) string {
	base := fmt.Sprintf("%s/%s/%s", owner, name, application)
	if providerSessionID == "" {
		return base
	}
	return base + "/" + providerSessionID
}

func parseCasdoorTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}
