// Package errorx defines Aisphere Kernel's shared error semantics.
//
// errorx is deliberately transport-neutral and depends only on the Go standard
// library. It is the foundation consumed by logx, httpx, grpcx, auditx,
// metricsx and workerx.
package errorx

import "regexp"

// Code is a stable, low-cardinality, machine-readable error code.
//
// Codes should use upper snake case. Business modules should define their own
// domain-specific constants, for example AIHUB_SKILL_NOT_FOUND.
type Code string

func (c Code) String() string { return string(c) }

const (
	CodeOK                  Code = "OK"
	CodeBadRequest          Code = "BAD_REQUEST"
	CodeUnauthorized        Code = "UNAUTHORIZED"
	CodeForbidden           Code = "FORBIDDEN"
	CodeNotFound            Code = "NOT_FOUND"
	CodeConflict            Code = "CONFLICT"
	CodeTooManyRequests     Code = "TOO_MANY_REQUESTS"
	CodeRequestTimeout      Code = "REQUEST_TIMEOUT"
	CodeClientClosedRequest Code = "CLIENT_CLOSED_REQUEST"
	CodeInternal            Code = "INTERNAL_ERROR"
	CodeUnavailable         Code = "SERVICE_UNAVAILABLE"
	CodeTimeout             Code = "TIMEOUT"
)

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`)

// IsValidCode reports whether code follows the stable upper-snake-case format.
// Empty code is invalid. OK is valid.
func IsValidCode(code Code) bool {
	return code != "" && codePattern.MatchString(code.String())
}

// NormalizeCode converts an empty code to INTERNAL_ERROR. It does not rewrite
// invalid non-empty business codes; callers should use IsValidCode in tests and
// lint rules to keep error-code contracts clean.
func NormalizeCode(code Code) Code {
	if code == "" {
		return CodeInternal
	}
	return code
}

func ValidateCode(code Code) error {
	if IsValidCode(code) {
		return nil
	}
	return New(CodeBadRequest, WithMessage("invalid error code"), WithMetadata("code", code.String()))
}

func MustCode(code string) Code {
	c := Code(code)
	if !IsValidCode(c) {
		panic("invalid error code: " + code)
	}
	return c
}

func MustValidCodes(codes ...Code) {
	for _, code := range codes {
		if !IsValidCode(code) {
			panic("invalid error code: " + code.String())
		}
	}
}
