package errorx

// Category is an optional coarse-grained error class for logs, metrics,
// alerting and AI-assisted troubleshooting.
type Category string

const (
	CategoryOK         Category = "ok"
	CategoryValidation Category = "validation"
	CategoryAuth       Category = "auth"
	CategoryPermission Category = "permission"
	CategoryNotFound   Category = "not_found"
	CategoryConflict   Category = "conflict"
	CategoryRateLimit  Category = "rate_limit"
	CategoryDependency Category = "dependency"
	CategoryCanceled   Category = "canceled"
	CategoryInternal   Category = "internal"
)

func (c Category) String() string { return string(c) }

func defaultCategory(code Code) Category {
	switch NormalizeCode(code) {
	case CodeOK:
		return CategoryOK
	case CodeBadRequest, CodeRequestTimeout:
		return CategoryValidation
	case CodeUnauthorized:
		return CategoryAuth
	case CodeForbidden:
		return CategoryPermission
	case CodeNotFound:
		return CategoryNotFound
	case CodeConflict:
		return CategoryConflict
	case CodeTooManyRequests:
		return CategoryRateLimit
	case CodeUnavailable, CodeTimeout:
		return CategoryDependency
	case CodeClientClosedRequest:
		return CategoryCanceled
	case CodeInternal:
		return CategoryInternal
	default:
		return CategoryInternal
	}
}
