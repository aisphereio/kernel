package errorx

const (
	HTTPStatusOK                  = 200
	HTTPStatusBadRequest          = 400
	HTTPStatusUnauthorized        = 401
	HTTPStatusForbidden           = 403
	HTTPStatusNotFound            = 404
	HTTPStatusConflict            = 409
	HTTPStatusRequestTimeout      = 408
	HTTPStatusTooManyRequests     = 429
	HTTPStatusClientClosedRequest = 499
	HTTPStatusInternalServerError = 500
	HTTPStatusServiceUnavailable  = 503
	HTTPStatusGatewayTimeout      = 504
)

func defaultHTTPStatus(code Code) int {
	switch NormalizeCode(code) {
	case CodeOK:
		return HTTPStatusOK
	case CodeBadRequest:
		return HTTPStatusBadRequest
	case CodeUnauthorized:
		return HTTPStatusUnauthorized
	case CodeForbidden:
		return HTTPStatusForbidden
	case CodeNotFound:
		return HTTPStatusNotFound
	case CodeConflict:
		return HTTPStatusConflict
	case CodeRequestTimeout:
		return HTTPStatusRequestTimeout
	case CodeTooManyRequests:
		return HTTPStatusTooManyRequests
	case CodeClientClosedRequest:
		return HTTPStatusClientClosedRequest
	case CodeUnavailable:
		return HTTPStatusServiceUnavailable
	case CodeTimeout:
		return HTTPStatusGatewayTimeout
	case CodeInternal:
		return HTTPStatusInternalServerError
	default:
		return HTTPStatusInternalServerError
	}
}

func codeFromHTTPStatus(status int) Code {
	switch status {
	case HTTPStatusOK:
		return CodeOK
	case HTTPStatusBadRequest:
		return CodeBadRequest
	case HTTPStatusUnauthorized:
		return CodeUnauthorized
	case HTTPStatusForbidden:
		return CodeForbidden
	case HTTPStatusNotFound:
		return CodeNotFound
	case HTTPStatusConflict:
		return CodeConflict
	case HTTPStatusRequestTimeout:
		return CodeRequestTimeout
	case HTTPStatusTooManyRequests:
		return CodeTooManyRequests
	case HTTPStatusClientClosedRequest:
		return CodeClientClosedRequest
	case HTTPStatusServiceUnavailable:
		return CodeUnavailable
	case HTTPStatusGatewayTimeout:
		return CodeTimeout
	default:
		return CodeInternal
	}
}

func isValidHTTPStatus(status int) bool {
	return status >= 100 && status <= 599
}
