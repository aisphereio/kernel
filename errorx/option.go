package errorx

// Option mutates Error during construction.
type Option func(*Error)

func WithMessage(message string) Option {
	return func(e *Error) { e.message = message }
}

func WithHTTPStatus(status int) Option {
	return func(e *Error) {
		if isValidHTTPStatus(status) {
			e.httpStatus = status
		}
	}
}

func WithStatusCode(status int) Option { return WithHTTPStatus(status) }

func WithGRPCCode(code string) Option {
	return func(e *Error) { e.grpcCode = code }
}

func WithRetryable(retryable bool) Option {
	return func(e *Error) { e.retryable = retryable }
}

func WithCause(cause error) Option {
	return func(e *Error) { e.cause = cause }
}

func WithMetadata(key string, value any) Option {
	return func(e *Error) {
		e.metadata = mergeMap(e.metadata, map[string]any{key: value})
	}
}

func WithMetadataMap(metadata map[string]any) Option {
	return func(e *Error) { e.metadata = mergeMap(e.metadata, metadata) }
}

// WithPublicMetadata marks a metadata key as safe to expose via transport
// responses. Secrets and large objects should never be put here.
func WithPublicMetadata(key string, value any) Option {
	return func(e *Error) {
		e.publicMetadata = mergeMap(e.publicMetadata, redactMetadata(map[string]any{key: value}))
	}
}

func WithPublicMetadataMap(metadata map[string]any) Option {
	return func(e *Error) { e.publicMetadata = mergeMap(e.publicMetadata, redactMetadata(metadata)) }
}

func WithRequestID(requestID string) Option {
	return func(e *Error) { e.requestID = requestID }
}

func WithTraceID(traceID string) Option {
	return func(e *Error) { e.traceID = traceID }
}

func WithCategory(category Category) Option {
	return func(e *Error) { e.category = category }
}

func WithSeverity(severity Severity) Option {
	return func(e *Error) { e.severity = severity }
}

// WithStack captures a call stack at construction time using only the standard
// library. The stack is intended for logs/debugging, not user-facing responses.
func WithStack() Option {
	return WithStackDepth(defaultStackDepth)
}

func WithStackDepth(depth int) Option {
	return func(e *Error) {
		// runtime.Callers -> captureStack -> option func -> constructor call site.
		e.stack = captureStack(4, depth)
	}
}
