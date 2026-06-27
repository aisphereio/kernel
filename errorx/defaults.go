package errorx

func defaultRetryable(code Code) bool {
	switch NormalizeCode(code) {
	case CodeTooManyRequests, CodeUnavailable, CodeTimeout:
		return true
	default:
		return false
	}
}

func defaultMessage(code Code) string {
	switch NormalizeCode(code) {
	case CodeOK:
		return "success"
	case CodeBadRequest:
		return "bad request"
	case CodeUnauthorized:
		return "unauthorized"
	case CodeForbidden:
		return "forbidden"
	case CodeNotFound:
		return "not found"
	case CodeConflict:
		return "conflict"
	case CodeRequestTimeout:
		return "request timeout"
	case CodeTooManyRequests:
		return "too many requests"
	case CodeClientClosedRequest:
		return "client closed request"
	case CodeUnavailable:
		return "service unavailable"
	case CodeTimeout:
		return "timeout"
	case CodeInternal:
		return "internal server error"
	default:
		return NormalizeCode(code).String()
	}
}
