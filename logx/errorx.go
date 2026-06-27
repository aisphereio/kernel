package logx

// Weak interfaces for future errorx integration. logx must not import errorx,
// otherwise errorx and logx can easily form an import cycle.
type codeCarrier interface{ Code() string }
type errorCodeCarrier interface{ ErrorCode() string }
type reasonCarrier interface{ Reason() string }
type httpStatusCarrier interface{ HTTPStatus() int }
type statusCodeCarrier interface{ StatusCode() int }
type retryableCarrier interface{ Retryable() bool }
type grpcCodeStringCarrier interface{ GRPCCode() string }
type grpcCodeIntCarrier interface{ GRPCCode() int }

// ErrorLog is a standard helper for business error logs.
type ErrorLog struct {
	Operation    string
	ResourceType string
	ResourceID   string
	Code         string
	Reason       string
	Err          error
	Fields       []Field
}

func LogError(logger Logger, msg string, e ErrorLog) {
	if logger == nil {
		logger = Noop()
	}
	fields := []Field{
		Event("error"),
		String("operation", e.Operation),
		String("resource_type", e.ResourceType),
		String("resource_id", e.ResourceID),
		String("error_code", e.Code),
		String("error_reason", e.Reason),
	}
	if e.Err != nil {
		fields = append(fields, Err(e.Err))
	}
	fields = append(fields, e.Fields...)
	logger.Error(msg, fields...)
}
