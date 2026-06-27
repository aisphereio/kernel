package errorx

import "fmt"

// CodedError is the minimum contract consumed by logx/httpx/auditx/metricsx.
//
// Other error implementations may satisfy this interface without depending on
// *errorx.Error directly.
type CodedError interface {
	error
	Code() Code
	Message() string
	HTTPStatus() int
	Retryable() bool
}

// Compatibility and extension contracts. Inspect helpers use errors.As against
// these interfaces, so wrapped third-party errors are also recognized.
type (
	ErrorCodeResponder  interface{ ErrorCode() string }
	MessageResponder    interface{ Message() string }
	HTTPStatusResponder interface {
		error
		HTTPStatus() int
	}
	StatusCodeResponder interface {
		error
		StatusCode() int
	}
	GRPCCodeResponder       interface{ GRPCCode() string }
	RetryableResponder      interface{ Retryable() bool }
	MetadataResponder       interface{ Metadata() map[string]any }
	PublicMetadataResponder interface{ PublicMetadata() map[string]any }
	RequestIDResponder      interface{ RequestID() string }
	TraceIDResponder        interface{ TraceID() string }
	CategoryResponder       interface{ Category() Category }
	SeverityResponder       interface{ Severity() Severity }
	StackResponder          interface{ Stack() []Frame }
)

// Error is Aisphere Kernel's canonical error implementation.
//
// Fields are deliberately unexported to keep the public API stable. Use methods
// and Option constructors instead of direct field access.
type Error struct {
	code           Code
	message        string
	httpStatus     int
	grpcCode       string
	retryable      bool
	cause          error
	metadata       map[string]any
	publicMetadata map[string]any
	requestID      string
	traceID        string
	category       Category
	severity       Severity
	stack          []uintptr
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is allows errors.Is to match another *Error or any coded error by code.
// Message and metadata are contextual and are intentionally ignored.
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	if te, ok := target.(*Error); ok {
		return e.Code() == te.Code()
	}
	if ce, ok := target.(interface{ Code() Code }); ok {
		return e.Code() == ce.Code()
	}
	if ce, ok := target.(ErrorCodeResponder); ok {
		return e.Code().String() == ce.ErrorCode()
	}
	return false
}

func (e *Error) Code() Code {
	if e == nil {
		return CodeInternal
	}
	return NormalizeCode(e.code)
}

// ErrorCode is a string-returning alias for compatibility with adapters and
// external frameworks that do not use errorx.Code.
func (e *Error) ErrorCode() string { return e.Code().String() }

func (e *Error) Message() string {
	if e == nil {
		return defaultMessage(CodeInternal)
	}
	if e.message != "" {
		return e.message
	}
	return defaultMessage(e.Code())
}

func (e *Error) HTTPStatus() int {
	if e == nil {
		return HTTPStatusInternalServerError
	}
	if e.httpStatus > 0 {
		return e.httpStatus
	}
	return defaultHTTPStatus(e.Code())
}

// StatusCode is provided for GoFr-style interoperability.
func (e *Error) StatusCode() int { return e.HTTPStatus() }

func (e *Error) GRPCCode() string {
	if e == nil {
		return defaultGRPCCode(CodeInternal)
	}
	if e.grpcCode != "" {
		return e.grpcCode
	}
	return defaultGRPCCode(e.Code())
}

func (e *Error) Retryable() bool {
	if e == nil {
		return false
	}
	return e.retryable
}

func (e *Error) Category() Category {
	if e == nil {
		return CategoryInternal
	}
	if e.category != "" {
		return e.category
	}
	return defaultCategory(e.Code())
}

func (e *Error) Severity() Severity {
	if e == nil {
		return SeverityError
	}
	if e.severity != "" {
		return e.severity
	}
	return defaultSeverityByStatus(e.HTTPStatus())
}

func (e *Error) RequestID() string {
	if e == nil {
		return ""
	}
	return e.requestID
}

func (e *Error) TraceID() string {
	if e == nil {
		return ""
	}
	return e.traceID
}

func (e *Error) Cause() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Metadata returns a defensive copy to avoid accidental mutation across layers.
func (e *Error) Metadata() map[string]any {
	if e == nil {
		return map[string]any{}
	}
	return cloneMap(e.metadata)
}

// PublicMetadata returns metadata that is explicitly safe for transport
// responses. httpx may choose to expose it depending on environment/config.
func (e *Error) PublicMetadata() map[string]any {
	if e == nil {
		return map[string]any{}
	}
	return cloneMap(e.publicMetadata)
}

func (e *Error) Stack() []Frame {
	if e == nil || len(e.stack) == 0 {
		return nil
	}
	return framesFromPCs(e.stack)
}

// Clone returns a deep copy of Error metadata while preserving the cause chain.
func (e *Error) Clone() *Error {
	if e == nil {
		return nil
	}
	stack := make([]uintptr, len(e.stack))
	copy(stack, e.stack)
	return &Error{
		code:           e.code,
		message:        e.message,
		httpStatus:     e.httpStatus,
		grpcCode:       e.grpcCode,
		retryable:      e.retryable,
		cause:          e.cause,
		metadata:       e.Metadata(),
		publicMetadata: e.PublicMetadata(),
		requestID:      e.requestID,
		traceID:        e.traceID,
		category:       e.category,
		severity:       e.severity,
		stack:          stack,
	}
}

// Format supports fmt.Printf("%+v", err) with useful debugging information while
// Error() stays safe for user-facing messages.
func (e *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "error_code=%s message=%q http_status=%d grpc_code=%s retryable=%t category=%s severity=%s request_id=%s trace_id=%s metadata=%v public_metadata=%v cause=%v",
				e.Code(), e.Message(), e.HTTPStatus(), e.GRPCCode(), e.Retryable(), e.Category(), e.Severity(), e.RequestID(), e.TraceID(), e.Metadata(), e.PublicMetadata(), e.Cause())
			return
		}
		fallthrough
	case 's':
		_, _ = fmt.Fprint(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}

// WithCause returns a cloned error with the supplied cause attached.
func (e *Error) WithCause(cause error) *Error {
	clone := e.Clone()
	if clone == nil {
		return nil
	}
	clone.cause = cause
	return clone
}

// WithMetadata returns a cloned error with internal metadata merged in.
func (e *Error) WithMetadata(metadata map[string]any) *Error {
	clone := e.Clone()
	if clone == nil {
		return nil
	}
	clone.metadata = mergeMap(clone.metadata, metadata)
	return clone
}

// WithPublicMetadata returns a cloned error with transport-safe metadata merged in.
func (e *Error) WithPublicMetadata(metadata map[string]any) *Error {
	clone := e.Clone()
	if clone == nil {
		return nil
	}
	clone.publicMetadata = mergeMap(clone.publicMetadata, redactMetadata(metadata))
	return clone
}
