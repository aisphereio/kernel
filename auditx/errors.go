package auditx

import "github.com/aisphereio/kernel/errorx"

const (
	CodeInvalidRecord = errorx.Code("AUDITX_INVALID_RECORD")
	CodeStoreFailed   = errorx.Code("AUDITX_STORE_FAILED")
)

func ErrInvalidRecord(message string) error {
	if message == "" {
		message = "invalid audit record"
	}
	return errorx.BadRequest(CodeInvalidRecord, message)
}

func ErrStoreFailed(message string, cause error) error {
	if message == "" {
		message = "audit store failed"
	}
	if cause == nil {
		return errorx.Unavailable(CodeStoreFailed, message)
	}
	return errorx.Wrap(cause, CodeStoreFailed, errorx.WithMessage(message))
}
