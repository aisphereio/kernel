package authz

import "github.com/aisphereio/kernel/errorx"

const (
	CodePermissionDenied      = errorx.Code("AUTHZ_PERMISSION_DENIED")
	CodeInvalidRequest        = errorx.Code("AUTHZ_INVALID_REQUEST")
	CodeRelationshipConflict  = errorx.Code("AUTHZ_RELATIONSHIP_CONFLICT")
	CodeRelationshipNotFound  = errorx.Code("AUTHZ_RELATIONSHIP_NOT_FOUND")
	CodeBackendFailed         = errorx.Code("AUTHZ_BACKEND_FAILED")
	CodeUnsupportedCapability = errorx.Code("AUTHZ_UNSUPPORTED_CAPABILITY")
)

func ErrPermissionDenied(message string) error {
	if message == "" {
		message = "permission denied"
	}
	return errorx.Forbidden(CodePermissionDenied, message)
}

func ErrInvalidRequest(message string) error {
	if message == "" {
		message = "invalid authorization request"
	}
	return errorx.BadRequest(CodeInvalidRequest, message)
}

func ErrBackendFailed(message string, cause error) error {
	if message == "" {
		message = "authorization backend failed"
	}
	if cause == nil {
		return errorx.Unavailable(CodeBackendFailed, message)
	}
	return errorx.Wrap(cause, CodeBackendFailed, errorx.WithMessage(message))
}

func ErrUnsupportedCapability(message string) error {
	if message == "" {
		message = "authorization capability is unsupported"
	}
	return errorx.BadRequest(CodeUnsupportedCapability, message)
}
