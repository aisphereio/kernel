package authn

import "github.com/aisphereio/kernel/errorx"

const (
	CodeMissingCredential       = errorx.Code("AUTHN_MISSING_CREDENTIAL")
	CodeInvalidCredential       = errorx.Code("AUTHN_INVALID_CREDENTIAL")
	CodeUnauthenticated         = errorx.Code("AUTHN_UNAUTHENTICATED")
	CodeInvalidTokenRequest     = errorx.Code("AUTHN_INVALID_TOKEN_REQUEST")
	CodeIdentityBackendFailed   = errorx.Code("AUTHN_IDENTITY_BACKEND_FAILED")
	CodeIdentityObjectNotFound  = errorx.Code("AUTHN_IDENTITY_OBJECT_NOT_FOUND")
	CodeIdentityObjectConflict  = errorx.Code("AUTHN_IDENTITY_OBJECT_CONFLICT")
	CodeUnsupportedIdentityFlow = errorx.Code("AUTHN_UNSUPPORTED_IDENTITY_FLOW")
)

func ErrMissingCredential(message string) error {
	if message == "" {
		message = "missing credential"
	}
	return errorx.Unauthorized(CodeMissingCredential, message)
}

func ErrInvalidCredential(message string) error {
	if message == "" {
		message = "invalid credential"
	}
	return errorx.Unauthorized(CodeInvalidCredential, message)
}

func ErrUnauthenticated(message string) error {
	if message == "" {
		message = "unauthenticated"
	}
	return errorx.Unauthorized(CodeUnauthenticated, message)
}

func ErrInvalidTokenRequest(message string) error {
	if message == "" {
		message = "invalid token request"
	}
	return errorx.BadRequest(CodeInvalidTokenRequest, message)
}

func ErrIdentityBackendFailed(message string, cause error) error {
	if message == "" {
		message = "identity backend failed"
	}
	if cause == nil {
		return errorx.Unavailable(CodeIdentityBackendFailed, message)
	}
	return errorx.Wrap(cause, CodeIdentityBackendFailed, errorx.WithMessage(message))
}
