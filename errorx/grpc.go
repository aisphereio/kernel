package errorx

// gRPC code names are represented as strings to keep errorx free of grpc-go.
// grpcx should translate these strings into google.golang.org/grpc/codes.Code.
const (
	GRPCCodeOK                = "OK"
	GRPCCodeCanceled          = "Canceled"
	GRPCCodeInvalidArgument   = "InvalidArgument"
	GRPCCodeDeadlineExceeded  = "DeadlineExceeded"
	GRPCCodeNotFound          = "NotFound"
	GRPCCodeAlreadyExists     = "AlreadyExists"
	GRPCCodePermissionDenied  = "PermissionDenied"
	GRPCCodeResourceExhausted = "ResourceExhausted"
	GRPCCodeInternal          = "Internal"
	GRPCCodeUnavailable       = "Unavailable"
	GRPCCodeUnauthenticated   = "Unauthenticated"
)

func defaultGRPCCode(code Code) string {
	switch NormalizeCode(code) {
	case CodeOK:
		return GRPCCodeOK
	case CodeBadRequest:
		return GRPCCodeInvalidArgument
	case CodeUnauthorized:
		return GRPCCodeUnauthenticated
	case CodeForbidden:
		return GRPCCodePermissionDenied
	case CodeNotFound:
		return GRPCCodeNotFound
	case CodeConflict:
		return GRPCCodeAlreadyExists
	case CodeRequestTimeout:
		return GRPCCodeDeadlineExceeded
	case CodeTooManyRequests:
		return GRPCCodeResourceExhausted
	case CodeClientClosedRequest:
		return GRPCCodeCanceled
	case CodeUnavailable:
		return GRPCCodeUnavailable
	case CodeTimeout:
		return GRPCCodeDeadlineExceeded
	case CodeInternal:
		return GRPCCodeInternal
	default:
		return GRPCCodeInternal
	}
}

func grpcCodeFromHTTPStatus(status int) string {
	return defaultGRPCCode(codeFromHTTPStatus(status))
}
