package iamx

import "github.com/aisphereio/kernel/errorx"

const (
	CodeInvalidArgument = "IAM_INVALID_ARGUMENT"
	CodeNotFound        = "IAM_NOT_FOUND"
	CodeConflict        = "IAM_CONFLICT"
)

func ErrInvalidArgument(message string) error { return errorx.BadRequest(CodeInvalidArgument, message) }
func ErrNotFound(message string) error        { return errorx.NotFound(CodeNotFound, message) }
func ErrConflict(message string) error        { return errorx.Conflict(CodeConflict, message) }
