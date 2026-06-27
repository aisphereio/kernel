package errorx

// New creates a canonical Error with default mappings derived from code.
func New(code Code, opts ...Option) *Error {
	code = NormalizeCode(code)
	e := &Error{
		code:           code,
		httpStatus:     defaultHTTPStatus(code),
		grpcCode:       defaultGRPCCode(code),
		retryable:      defaultRetryable(code),
		category:       defaultCategory(code),
		severity:       defaultSeverityByStatus(defaultHTTPStatus(code)),
		metadata:       map[string]any{},
		publicMetadata: map[string]any{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

// Wrap annotates cause with Kernel error semantics. If cause is nil, Wrap returns nil.
func Wrap(cause error, code Code, opts ...Option) error {
	if cause == nil {
		return nil
	}
	all := make([]Option, 0, len(opts)+1)
	all = append(all, WithCause(cause))
	all = append(all, opts...)
	return New(code, all...)
}

// From converts err to *Error. Kernel errors are cloned. Foreign errors are
// converted using supported interfaces such as Code/ErrorCode/HTTPStatus/StatusCode.
func From(err error, opts ...Option) *Error {
	if err == nil {
		return nil
	}
	if e, ok := As(err); ok {
		clone := e.Clone()
		for _, opt := range opts {
			if opt != nil {
				opt(clone)
			}
		}
		return clone
	}
	code := CodeOf(err)
	if code == CodeInternal {
		if status := HTTPStatusOf(err); status != HTTPStatusInternalServerError {
			code = codeFromHTTPStatus(status)
		}
	}
	all := []Option{
		WithMessage(MessageOf(err)),
		WithHTTPStatus(HTTPStatusOf(err)),
		WithGRPCCode(GRPCCodeOf(err)),
		WithRetryable(RetryableOf(err)),
		WithCause(err),
		WithMetadataMap(MetadataOf(err)),
		WithPublicMetadataMap(PublicMetadataOf(err)),
		WithRequestID(RequestIDOf(err)),
		WithTraceID(TraceIDOf(err)),
		WithCategory(CategoryOf(err)),
		WithSeverity(SeverityOf(err)),
	}
	all = append(all, opts...)
	return New(code, all...)
}

// NewStatus creates an Error using an explicit HTTP status while keeping
// derived gRPC/category/severity/retryability values consistent.
// It is primarily used by generated proto errorx helpers where the stable
// business code comes from an enum value and the transport status comes from
// proto options.
func NewStatus(code Code, httpStatus int, message string, opts ...Option) *Error {
	return newStatus(code, message, httpStatus, opts...)
}

func BadRequest(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusBadRequest, opts...)
}

func Unauthorized(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusUnauthorized, opts...)
}

func Forbidden(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusForbidden, opts...)
}

func NotFound(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusNotFound, opts...)
}

func Conflict(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusConflict, opts...)
}

func RequestTimeout(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusRequestTimeout, opts...)
}

func TooManyRequests(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusTooManyRequests, opts...)
}

func ClientClosedRequest(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusClientClosedRequest, opts...)
}

// ClientClosed is a short alias for ClientClosedRequest.
func ClientClosed(code Code, message string, opts ...Option) *Error {
	return ClientClosedRequest(code, message, opts...)
}

func Internal(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusInternalServerError, opts...)
}

// InternalServer is an alias that mirrors Kratos naming.
func InternalServer(code Code, message string, opts ...Option) *Error {
	return Internal(code, message, opts...)
}

func Unavailable(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusServiceUnavailable, opts...)
}

func Timeout(code Code, message string, opts ...Option) *Error {
	return newStatus(code, message, HTTPStatusGatewayTimeout, opts...)
}

// GatewayTimeout is an alias that mirrors Kratos naming.
func GatewayTimeout(code Code, message string, opts ...Option) *Error {
	return Timeout(code, message, opts...)
}

func newStatus(code Code, message string, status int, opts ...Option) *Error {
	base := []Option{
		WithMessage(message),
		WithHTTPStatus(status),
		WithGRPCCode(grpcCodeFromHTTPStatus(status)),
		WithRetryable(defaultRetryable(codeFromHTTPStatus(status))),
		WithCategory(defaultCategory(codeFromHTTPStatus(status))),
		WithSeverity(defaultSeverityByStatus(status)),
	}
	base = append(base, opts...)
	return New(code, base...)
}
