package http

import (
	"bytes"
	stderrors "errors"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/gorilla/mux"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/proto"

	"github.com/aisphereio/kernel/encoding"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/internal/httputil"
)

// SupportPackageIsVersion3 These constants should not be referenced from any other code.
const SupportPackageIsVersion3 = true

const defaultHTTPBodyContentType = "application/octet-stream"

var protoMessageType = reflect.TypeOf((*proto.Message)(nil)).Elem()

// Redirector replies to the request with a redirect to url
// which may be a path relative to the request path.
type Redirector interface {
	error
	Redirect() (string, int)
}

// Request type net/http.
type Request = http.Request

// ResponseWriter type net/http.
type ResponseWriter = http.ResponseWriter

// Flusher type net/http
type Flusher = http.Flusher

// DecodeRequestFunc is decode request func.
type DecodeRequestFunc func(*http.Request, any) error

// EncodeResponseFunc is encode response func.
type EncodeResponseFunc func(http.ResponseWriter, *http.Request, any) error

// EncodeErrorFunc is encode error func.
type EncodeErrorFunc func(http.ResponseWriter, *http.Request, error)

// DefaultRequestVars decodes the request vars to object.
func DefaultRequestVars(r *http.Request, v any) error {
	raws := mux.Vars(r)
	vars := make(url.Values, len(raws))
	for k, v := range raws {
		vars[k] = []string{v}
	}
	return bindQuery(vars, v)
}

// DefaultRequestQuery decodes the request vars to object.
func DefaultRequestQuery(r *http.Request, v any) error {
	return bindQuery(r.URL.Query(), v)
}

// DefaultRequestDecoder decodes the request body to object.
func DefaultRequestDecoder(r *http.Request, v any) error {
	if body, ok := httpBody(v); ok {
		data, err := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(data))
		if err != nil {
			return errorx.BadRequest("REQUEST_BODY_READ_FAILED", "request body read failed", errorx.WithCause(err), errorx.WithMetadata("read_error", err.Error()))
		}
		body.ContentType = r.Header.Get("Content-Type")
		body.Data = data
		return nil
	}
	codec, ok := CodecForRequest(r, "Content-Type")
	if !ok {
		return errorx.BadRequest("REQUEST_CONTENT_TYPE_UNSUPPORTED", "unsupported request content type", errorx.WithMetadata("content_type", r.Header.Get("Content-Type")))
	}
	data, err := io.ReadAll(r.Body)

	// reset body.
	r.Body = io.NopCloser(bytes.NewBuffer(data))

	if err != nil {
		return errorx.BadRequest("REQUEST_BODY_READ_FAILED", "request body read failed", errorx.WithCause(err), errorx.WithMetadata("read_error", err.Error()))
	}
	if len(data) == 0 {
		return nil
	}
	if err = decodeWithCodec(codec, data, v); err != nil {
		return errorx.BadRequest("REQUEST_BODY_DECODE_FAILED", "request body decode failed", errorx.WithCause(err), errorx.WithMetadata("decode_error", err.Error()))
	}
	return nil
}

// DefaultResponseEncoder encodes the object to the HTTP response.
func DefaultResponseEncoder(w http.ResponseWriter, r *http.Request, v any) error {
	if v == nil {
		return nil
	}
	if body, ok := httpBody(v); ok {
		contentType := body.GetContentType()
		if contentType == "" {
			contentType = defaultHTTPBodyContentType
		}
		w.Header().Set("Content-Type", contentType)
		_, err := w.Write(body.GetData())
		return err
	}
	if rd, ok := v.(Redirector); ok {
		url, code := rd.Redirect()
		http.Redirect(w, r, url, code)
		return nil
	}
	codec, _ := CodecForRequest(r, "Accept")
	data, err := codec.Marshal(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", httputil.ContentType(codec.Name()))
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// DefaultErrorEncoder encodes the error to the HTTP response.
func DefaultErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
	var rd *redirect
	if stderrors.As(err, &rd) {
		url, code := rd.Redirect()
		http.Redirect(w, r, url, code)
		return
	}
	ke := errorx.From(err)
	resp := ErrorResponseFromError(ke)
	codec, _ := CodecForRequest(r, "Accept")
	if codec.Name() == "proto" || codec.Name() == "protojson" {
		codec = encoding.GetCodec("json")
	}
	body, marshalErr := codec.Marshal(resp)
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", httputil.ContentType(codec.Name()))
	w.WriteHeader(ke.HTTPStatus())
	_, _ = w.Write(body)
}

// ErrorResponse is the transport-safe HTTP representation of an errorx error.
// HTTP status stays in the response status code; Code is the stable business code.
type ErrorResponse struct {
	Code      string         `json:"code" yaml:"code"`
	Message   string         `json:"message" yaml:"message"`
	RequestID string         `json:"request_id,omitempty" yaml:"request_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func ErrorResponseFromError(err error) ErrorResponse {
	ke := errorx.From(err)
	if ke == nil {
		ke = errorx.Internal(errorx.CodeInternal, "internal server error")
	}
	return ErrorResponse{
		Code:      ke.Code().String(),
		Message:   ke.Message(),
		RequestID: ke.RequestID(),
		TraceID:   ke.TraceID(),
		Metadata:  ke.PublicMetadata(),
	}
}

// CodecForRequest get encoding.Codec via http.Request
func CodecForRequest(r *http.Request, name string) (encoding.Codec, bool) {
	for _, accept := range r.Header[name] {
		codec := encoding.GetCodec(httputil.ContentSubtype(accept))
		if codec != nil {
			return codec, true
		}
	}
	return encoding.GetCodec("json"), false
}

func httpBody(v any) (*httpbody.HttpBody, bool) {
	switch body := v.(type) {
	case *httpbody.HttpBody:
		return body, body != nil
	case **httpbody.HttpBody:
		if body == nil {
			return nil, false
		}
		if *body == nil {
			*body = new(httpbody.HttpBody)
		}
		return *body, true
	default:
		return nil, false
	}
}

func decodeWithCodec(codec encoding.Codec, data []byte, v any) error {
	switch codec.Name() {
	case "proto", "protojson":
	default:
		return codec.Unmarshal(data, v)
	}

	if msg, ok := v.(proto.Message); ok {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			return codec.Unmarshal(data, v)
		}
		return codec.Unmarshal(data, msg)
	}

	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
		return codec.Unmarshal(data, v)
	}

	elem := rv.Type().Elem()
	if elem.Kind() != reflect.Pointer || !elem.Implements(protoMessageType) {
		return codec.Unmarshal(data, v)
	}

	target := rv.Elem()
	if target.IsNil() {
		target.Set(reflect.New(elem.Elem()))
	}
	return codec.Unmarshal(data, target.Interface())
}

// BodyContentType returns the content type carried by v or a binary default.
func BodyContentType(v any) string {
	if body, ok := httpBody(v); ok && body.GetContentType() != "" {
		return body.GetContentType()
	}
	return defaultHTTPBodyContentType
}
