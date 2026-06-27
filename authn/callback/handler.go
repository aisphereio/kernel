// Package callback contains reusable HTTP helpers for browser based authn
// callback flows. It depends only on Kernel authn contracts, not on Casdoor SDK.
package callback

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

// Handler exchanges OAuth/OIDC callback code values through authn.LoginService.
// It is provider-neutral: Casdoor is only one possible implementation behind
// the service interface.
type Handler struct {
	Login  authn.LoginService
	Logger logx.Logger
	// LoginURL is optional. When it is set, a browser request to the callback
	// endpoint without a code is redirected to the provider login URL instead of
	// returning a raw JSON error. This makes local integration tests and real web
	// app flows friendlier while keeping the provider hidden behind authn.LoginService.
	LoginURL      string
	RedirectURI   string
	ExpectedState string
	OrgID         string
	AppID         string
	SuccessText   string
	OnSuccess     func(context.Context, authn.CallbackResult) error
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, errorx.BadRequest(errorx.CodeBadRequest, "authn callback only supports GET"))
		return
	}
	if h.Login == nil {
		h.writeError(w, http.StatusServiceUnavailable, errorx.Unavailable(errorx.CodeUnavailable, "authn login service is not configured"))
		return
	}
	providerErr := r.URL.Query().Get("error")
	if providerErr != "" {
		err := errorx.Unauthorized(errorx.CodeUnauthorized, "identity provider returned an error",
			errorx.WithMetadata("provider_error", providerErr),
			errorx.WithMetadata("provider_error_description", r.URL.Query().Get("error_description")),
		)
		h.logWarn("authn callback provider error", err)
		h.writeError(w, http.StatusUnauthorized, err)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" {
		if strings.TrimSpace(h.LoginURL) != "" {
			h.logInfo("authn callback missing code; redirecting to login", logx.String("login_url", h.LoginURL))
			http.Redirect(w, r, h.LoginURL, http.StatusFound)
			return
		}
		err := errorx.BadRequest(errorx.CodeBadRequest, "authn callback missing code")
		h.logWarn("authn callback missing code", err)
		h.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.Login.HandleCallback(r.Context(), authn.CallbackRequest{
		Code:          code,
		State:         state,
		ExpectedState: h.ExpectedState,
		RedirectURI:   h.RedirectURI,
		OrgID:         h.OrgID,
		AppID:         h.AppID,
	})
	if err != nil {
		err = errorx.Wrap(err, errorx.CodeUnauthorized, errorx.WithMessage("authn callback exchange failed"))
		h.logWarn("authn callback exchange failed", err)
		h.writeError(w, http.StatusUnauthorized, err)
		return
	}
	if h.OnSuccess != nil {
		if err := h.OnSuccess(r.Context(), result); err != nil {
			err = errorx.Wrap(err, errorx.CodeInternal, errorx.WithMessage("authn callback success hook failed"))
			h.logWarn("authn callback success hook failed", err)
			h.writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	h.logInfo("authn callback success",
		logx.String("subject_id", result.Principal.SubjectID),
		logx.String("username", result.Principal.Username),
		logx.String("org", result.Principal.OrgID),
		logx.String("provider", result.Provider),
		logx.Bool("has_access_token", result.Tokens.AccessToken != ""),
		logx.Bool("has_refresh_token", result.Tokens.RefreshToken != ""),
	)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"message": firstNonEmpty(h.SuccessText, "authentication callback completed; you can close this page"),
		"subject": result.Principal.SubjectID,
		"user":    result.Principal.Username,
		"org":     result.Principal.OrgID,
	})
}

func (h *Handler) logInfo(msg string, fields ...logx.Field) {
	if h.Logger != nil {
		h.Logger.Info(msg, fields...)
	}
}

func (h *Handler) logWarn(msg string, err error, fields ...logx.Field) {
	fields = append([]logx.Field{logx.Err(err), logx.String("error_code", errorx.CodeOf(err).String())}, fields...)
	if h.Logger != nil {
		h.Logger.Warn(msg, fields...)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "error",
		"error_code": errorx.CodeOf(err).String(),
		"message":    errorx.MessageOf(err),
		"time":       time.Now().Format(time.RFC3339),
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
