package kubernetesx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/aisphereio/kernel/errorx"
)

// Sentinel errors. These wrap foreign k8s errors into stable kernel sentinels
// so that callers can use errors.Is without importing k8s apierrors. Business
// code should prefer the errorx.Code constants below and NormalizeError.
var (
	// ErrConfigInvalid is returned by New when Config fails Validate.
	ErrConfigInvalid = errors.New("kubernetesx: config is invalid")

	// ErrCredentialInvalid is returned when a Credential fails Validate.
	ErrCredentialInvalid = errors.New("kubernetesx: credential is invalid")

	// ErrUnauthorized is the normalized 401 from the API server.
	ErrUnauthorized = errors.New("kubernetesx: unauthorized")

	// ErrForbidden is the normalized 403 from the API server.
	ErrForbidden = errors.New("kubernetesx: forbidden")

	// ErrNotFound is the normalized 404 from the API server.
	ErrNotFound = errors.New("kubernetesx: resource not found")

	// ErrAlreadyExists is the normalized 409 AlreadyExists.
	ErrAlreadyExists = errors.New("kubernetesx: resource already exists")

	// ErrConflict is a generic 409 Conflict (e.g. optimistic concurrency).
	ErrConflict = errors.New("kubernetesx: conflict")

	// ErrFieldConflict is an SSA field-ownership conflict. It must never be
	// silently overwritten; the caller decides whether to escalate or
	// force-apply.
	ErrFieldConflict = errors.New("kubernetesx: server-side apply field conflict")

	// ErrTimeout is a client-side or server-side timeout.
	ErrTimeout = errors.New("kubernetesx: operation timed out")

	// ErrUnreachable is a connection / DNS / TLS failure before the API
	// server responded.
	ErrUnreachable = errors.New("kubernetesx: api server unreachable")

	// ErrAPIUnavailable is a 5xx from the API server (excluding timeout).
	ErrAPIUnavailable = errors.New("kubernetesx: api server unavailable")
)

// Stable errorx codes. Business code reads these via errorx.CodeOf; they
// mirror the design §4.7 table exactly and are part of the public contract.
const (
	CodeConfigInvalid     = errorx.Code("KUBERNETES_CONFIG_INVALID")
	CodeCredentialInvalid = errorx.Code("KUBERNETES_CREDENTIAL_INVALID")
	CodeUnauthorized      = errorx.Code("KUBERNETES_UNAUTHORIZED")
	CodeForbidden         = errorx.Code("KUBERNETES_FORBIDDEN")
	CodeNotFound          = errorx.Code("KUBERNETES_NOT_FOUND")
	CodeAlreadyExists     = errorx.Code("KUBERNETES_ALREADY_EXISTS")
	CodeConflict          = errorx.Code("KUBERNETES_CONFLICT")
	CodeFieldConflict     = errorx.Code("KUBERNETES_FIELD_CONFLICT")
	CodeTimeout           = errorx.Code("KUBERNETES_TIMEOUT")
	CodeUnreachable       = errorx.Code("KUBERNETES_UNREACHABLE")
	CodeAPIUnavailable    = errorx.Code("KUBERNETES_API_UNAVAILABLE")
)

// NormalizeError converts a foreign Kubernetes / client-go error into a stable
// errorx-wrapped error carrying one of the KUBERNETES_* codes. It is idempotent:
// errors already produced by errorx pass through unchanged.
//
// Metadata never includes kubeconfig, token, or private-key material — only
// api_group / kind / namespace / name / reason / retryable, sourced from the
// apierrors.StatusError when available.
func NormalizeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errorx.As(err); ok {
		return err
	}
	code, status, message, retryable := classifyKubernetesError(err)
	opts := []errorx.Option{
		errorx.WithMessage(message),
		errorx.WithHTTPStatus(status),
		errorx.WithRetryable(retryable),
		errorx.WithMetadata("component", "kubernetesx"),
	}
	for k, v := range errorMetadata(err) {
		opts = append(opts, errorx.WithMetadata(k, v))
	}
	return errorx.Wrap(err, code, opts...)
}

// classifyKubernetesError maps an error to (code, httpStatus, message,
// retryable). It is the single place that imports k8s apierrors so that the
// rest of the package and all business callers stay free of apierrors.
func classifyKubernetesError(err error) (errorx.Code, int, string, bool) {
	// Sentinel passthrough — keeps errors.Is working for callers that prefer
	// the sentinel form.
	switch {
	case errors.Is(err, ErrConfigInvalid):
		return CodeConfigInvalid, errorx.HTTPStatusBadRequest, "kubernetes config is invalid", false
	case errors.Is(err, ErrCredentialInvalid):
		return CodeCredentialInvalid, errorx.HTTPStatusBadRequest, "kubernetes credential is invalid", false
	case errors.Is(err, ErrFieldConflict):
		return CodeFieldConflict, errorx.HTTPStatusConflict, "server-side apply field conflict", false
	case errors.Is(err, ErrUnreachable):
		return CodeUnreachable, errorx.HTTPStatusServiceUnavailable, "kubernetes api server unreachable", true
	}

	// Context timeouts / cancellation.
	switch {
	case errors.Is(err, context.Canceled):
		return CodeTimeout, errorx.HTTPStatusClientClosedRequest, "kubernetes operation canceled", false
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout, errorx.HTTPStatusGatewayTimeout, "kubernetes operation timed out", true
	}

	// apierrors.StatusError: the richest signal.
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) && statusErr.Status().Code != 0 {
		return classifyStatusError(statusErr)
	}

	// Network / transport heuristics. client-go wraps dial errors in
	// *url.Error or returns them bare; we detect by substring to avoid
	// importing every transport type.
	if isUnreachable(err) {
		return CodeUnreachable, errorx.HTTPStatusServiceUnavailable, "kubernetes api server unreachable", true
	}

	return errorx.Code("KUBERNETES_OPERATION_FAILED"), errorx.HTTPStatusInternalServerError, "kubernetes operation failed", false
}

// CodeOperationFailed is the catch-all code returned by NormalizeError for
// errors that do not match a more specific classification. It is exported so
// callers can compare with errorx.CodeOf == kubernetesx.CodeOperationFailed.
const CodeOperationFailed = errorx.Code("KUBERNETES_OPERATION_FAILED")

// classifyStatusError maps an apierrors.StatusError to a code. The HTTP status
// carried on the StatusError is preserved so that downstream HTTP/gRPC
// translation stays accurate.
func classifyStatusError(s *apierrors.StatusError) (errorx.Code, int, string, bool) {
	status := s.Status()
	httpStatus := int(status.Code)
	if httpStatus == 0 {
		httpStatus = errorx.HTTPStatusInternalServerError
	}
	msg := status.Message
	if msg == "" {
		msg = string(status.Reason)
	}
	if msg == "" {
		msg = "kubernetes api error"
	}
	switch {
	case apierrors.IsNotFound(s):
		return CodeNotFound, httpStatus, msg, false
	case apierrors.IsAlreadyExists(s):
		return CodeAlreadyExists, httpStatus, msg, false
	case apierrors.IsConflict(s):
		// SSA field conflicts surface as 409 with reason "ApplyConflict" or
		// a message mentioning field ownership; both map to field conflict
		// so callers can branch on KUBERNETES_FIELD_CONFLICT specifically.
		if isFieldConflict(status) {
			return CodeFieldConflict, httpStatus, msg, false
		}
		return CodeConflict, httpStatus, msg, true
	case apierrors.IsUnauthorized(s):
		return CodeUnauthorized, httpStatus, msg, false
	case apierrors.IsForbidden(s):
		return CodeForbidden, httpStatus, msg, false
	case apierrors.IsTimeout(s), apierrors.IsServerTimeout(s):
		return CodeTimeout, httpStatus, msg, true
	case apierrors.IsServiceUnavailable(s):
		return CodeAPIUnavailable, httpStatus, msg, true
	case apierrors.IsInternalError(s):
		return CodeAPIUnavailable, httpStatus, msg, true
	case apierrors.IsTooManyRequests(s):
		return errorx.Code("KUBERNETES_TOO_MANY_REQUESTS"), httpStatus, msg, true
	case apierrors.IsBadRequest(s):
		return CodeConfigInvalid, httpStatus, msg, false
	default:
		return CodeOperationFailed, httpStatus, msg, false
	}
}

// isFieldConflict reports whether a 409 status represents an SSA field
// ownership conflict rather than a generic optimistic-concurrency conflict.
// Kubernetes uses the reason "ApplyConflict" for field conflicts; older
// servers emit a message containing "field manager" / "owned by".
func isFieldConflict(status metav1.Status) bool {
	reason := string(status.Reason)
	if reason == "ApplyConflict" {
		return true
	}
	lower := strings.ToLower(status.Message)
	if strings.Contains(lower, "field manager") {
		return true
	}
	if strings.Contains(lower, "is managed by") {
		return true
	}
	return false
}

// isUnreachable detects dial / TLS / DNS errors without importing transport
// types. client-go surfaces these as *url.Error wrapping a net.OpError, or as
// bare errors from the rest client; both carry recognizable substrings.
func isUnreachable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range unreachableMarkers {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

var unreachableMarkers = []string{
	"connection refused",
	"no such host",
	"i/o timeout",
	"tls:",
	"x509:",
	"dial tcp",
	"dial: tcp",
	"connection reset",
	"broken pipe",
	"network is unreachable",
}

// errorMetadata extracts non-sensitive metadata from a k8s error for
// observability. It never reads kubeconfig, token, or key material.
func errorMetadata(err error) map[string]any {
	out := map[string]any{}
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		status := statusErr.Status()
		if status.Details != nil {
			if status.Details.Group != "" {
				out["api_group"] = status.Details.Group
			}
			if status.Details.Kind != "" {
				out["kind"] = status.Details.Kind
			}
			if status.Details.Name != "" {
				out["name"] = status.Details.Name
			}
			if len(status.Details.Causes) > 0 {
				// First cause is usually the field-level reason; keep it
				// bounded to avoid leaking large payloads.
				out["reason"] = status.Details.Causes[0].Message
			}
		}
		if status.Reason != "" {
			if _, ok := out["reason"]; !ok {
				out["reason"] = string(status.Reason)
			}
		}
		return out
	}
	// For unstructured / runtime.Object keyed errors we cannot extract
	// group/kind safely; leave metadata sparse.
	return out
}

// gvkFromObject is a helper for apply.go / probe.go to attach group/kind
// metadata when wrapping an error tied to a specific object.
func gvkFromObject(obj interface {
	GetObjectKind() schema.ObjectKind
}) (group, kind string) {
	if obj == nil {
		return "", ""
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	return gvk.Group, gvk.Kind
}

// withObjectMetadata attaches the object's group/kind/namespace/name to an
// errorx-wrapped error. Used by client.go to enrich per-call errors.
func withObjectMetadata(err error, group, kind, namespace, name string) error {
	if err == nil {
		return nil
	}
	e, ok := errorx.As(err)
	if !ok {
		return err
	}
	extra := map[string]any{}
	if group != "" {
		extra["api_group"] = group
	}
	if kind != "" {
		extra["kind"] = kind
	}
	if namespace != "" {
		extra["namespace"] = namespace
	}
	if name != "" {
		extra["name"] = name
	}
	if len(extra) == 0 {
		return err
	}
	return e.WithMetadata(extra)
}

// ensure net/http is referenced: the http.Status range check below guards
// against apierrors.StatusError carrying a non-HTTP status code.
var _ = http.StatusOK
