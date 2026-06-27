package errorx

import (
	"errors"
	"reflect"
)

// As extracts the canonical *Error from an error chain.
func As(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func IsKernelError(err error) bool {
	_, ok := As(err)
	return ok
}

func CodeOf(err error) Code {
	if err == nil {
		return CodeOK
	}
	if e, ok := As(err); ok {
		return e.Code()
	}
	var ce interface{ Code() Code }
	if errors.As(err, &ce) {
		return NormalizeCode(ce.Code())
	}
	var ec ErrorCodeResponder
	if errors.As(err, &ec) {
		return NormalizeCode(Code(ec.ErrorCode()))
	}
	if status := statusOfForeign(err); status > 0 && status != HTTPStatusInternalServerError {
		return codeFromHTTPStatus(status)
	}
	return CodeInternal
}

func MessageOf(err error) string {
	if err == nil {
		return defaultMessage(CodeOK)
	}
	if e, ok := As(err); ok {
		return e.Message()
	}
	var mr MessageResponder
	if errors.As(err, &mr) && mr.Message() != "" {
		return mr.Message()
	}
	// Foreign typed HTTP errors, such as GoFr errors, usually put their safe
	// response message in Error(). Plain errors.New/fmt.Errorf remain hidden.
	var hs HTTPStatusResponder
	if errors.As(err, &hs) && hs.HTTPStatus() > 0 && hs.HTTPStatus() < HTTPStatusInternalServerError {
		return hs.Error()
	}
	var sc StatusCodeResponder
	if errors.As(err, &sc) && sc.StatusCode() > 0 && sc.StatusCode() < HTTPStatusInternalServerError {
		return sc.Error()
	}
	return defaultMessage(CodeInternal)
}

func HTTPStatusOf(err error) int {
	if err == nil {
		return HTTPStatusOK
	}
	if e, ok := As(err); ok {
		return e.HTTPStatus()
	}
	if status := statusOfForeign(err); status > 0 {
		return status
	}
	return defaultHTTPStatus(CodeOf(err))
}

func GRPCCodeOf(err error) string {
	if err == nil {
		return GRPCCodeOK
	}
	if e, ok := As(err); ok {
		return e.GRPCCode()
	}
	var gc GRPCCodeResponder
	if errors.As(err, &gc) && gc.GRPCCode() != "" {
		return gc.GRPCCode()
	}
	return grpcCodeFromHTTPStatus(HTTPStatusOf(err))
}

func RetryableOf(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := As(err); ok {
		return e.Retryable()
	}
	var rr RetryableResponder
	if errors.As(err, &rr) {
		return rr.Retryable()
	}
	return defaultRetryable(CodeOf(err))
}

func CategoryOf(err error) Category {
	if err == nil {
		return CategoryOK
	}
	if e, ok := As(err); ok {
		return e.Category()
	}
	var cr CategoryResponder
	if errors.As(err, &cr) && cr.Category() != "" {
		return cr.Category()
	}
	return defaultCategory(CodeOf(err))
}

func SeverityOf(err error) Severity {
	if err == nil {
		return SeverityInfo
	}
	if e, ok := As(err); ok {
		return e.Severity()
	}
	var sr SeverityResponder
	if errors.As(err, &sr) && sr.Severity() != "" {
		return sr.Severity()
	}
	if sev, ok := severityFromForeignLogLevel(err); ok {
		return sev
	}
	return defaultSeverityByStatus(HTTPStatusOf(err))
}

func MetadataOf(err error) map[string]any {
	if err == nil {
		return map[string]any{}
	}
	if e, ok := As(err); ok {
		return e.Metadata()
	}
	var mr MetadataResponder
	if errors.As(err, &mr) {
		return cloneMap(mr.Metadata())
	}
	if md, ok := metadataField(err, "Metadata"); ok {
		return md
	}
	return map[string]any{}
}

func SafeMetadataOf(err error) map[string]any {
	return redactMetadata(MetadataOf(err))
}

func PublicMetadataOf(err error) map[string]any {
	if err == nil {
		return map[string]any{}
	}
	if e, ok := As(err); ok {
		return e.PublicMetadata()
	}
	var pr PublicMetadataResponder
	if errors.As(err, &pr) {
		return cloneMap(pr.PublicMetadata())
	}
	return map[string]any{}
}

func RequestIDOf(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := As(err); ok {
		return e.RequestID()
	}
	var rr RequestIDResponder
	if errors.As(err, &rr) {
		return rr.RequestID()
	}
	if v, ok := metadataString(MetadataOf(err), "request_id"); ok {
		return v
	}
	return ""
}

func TraceIDOf(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := As(err); ok {
		return e.TraceID()
	}
	var tr TraceIDResponder
	if errors.As(err, &tr) {
		return tr.TraceID()
	}
	if v, ok := metadataString(MetadataOf(err), "trace_id"); ok {
		return v
	}
	return ""
}

func StackOf(err error) []Frame {
	if err == nil {
		return nil
	}
	if e, ok := As(err); ok {
		return e.Stack()
	}
	var sr StackResponder
	if errors.As(err, &sr) {
		return sr.Stack()
	}
	return nil
}

func CauseOf(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := As(err); ok {
		return e.Cause()
	}
	return errors.Unwrap(err)
}

func IsCode(err error, code Code) bool { return CodeOf(err) == NormalizeCode(code) }

func IsBadRequest(err error) bool      { return HTTPStatusOf(err) == HTTPStatusBadRequest }
func IsUnauthorized(err error) bool    { return HTTPStatusOf(err) == HTTPStatusUnauthorized }
func IsForbidden(err error) bool       { return HTTPStatusOf(err) == HTTPStatusForbidden }
func IsNotFound(err error) bool        { return HTTPStatusOf(err) == HTTPStatusNotFound }
func IsConflict(err error) bool        { return HTTPStatusOf(err) == HTTPStatusConflict }
func IsRequestTimeout(err error) bool  { return HTTPStatusOf(err) == HTTPStatusRequestTimeout }
func IsTooManyRequests(err error) bool { return HTTPStatusOf(err) == HTTPStatusTooManyRequests }
func IsClientClosedRequest(err error) bool {
	return HTTPStatusOf(err) == HTTPStatusClientClosedRequest
}
func IsInternal(err error) bool    { return HTTPStatusOf(err) == HTTPStatusInternalServerError }
func IsUnavailable(err error) bool { return HTTPStatusOf(err) == HTTPStatusServiceUnavailable }
func IsTimeout(err error) bool     { return HTTPStatusOf(err) == HTTPStatusGatewayTimeout }

func statusOfForeign(err error) int {
	var hs HTTPStatusResponder
	if errors.As(err, &hs) && hs.HTTPStatus() > 0 {
		return hs.HTTPStatus()
	}
	var sc StatusCodeResponder
	if errors.As(err, &sc) && sc.StatusCode() > 0 {
		return sc.StatusCode()
	}
	return 0
}

func metadataString(md map[string]any, key string) (string, bool) {
	v, ok := md[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

func metadataField(err error, name string) (map[string]any, bool) {
	v := reflect.ValueOf(err)
	if !v.IsValid() {
		return nil, false
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}
	field := v.FieldByName(name)
	if !field.IsValid() || field.IsZero() || field.Kind() != reflect.Map || field.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(map[string]any, field.Len())
	iter := field.MapRange()
	for iter.Next() {
		k := iter.Key().String()
		if !iter.Value().CanInterface() {
			continue
		}
		out[k] = iter.Value().Interface()
	}
	return out, true
}
