package restx

import (
	"context"

	"buf.build/go/protovalidate"
	"github.com/aisphereio/kernel/errorx"
	httpx "github.com/aisphereio/kernel/transportx/http"
	"google.golang.org/protobuf/proto"
)

// InstallProtovalidateRequestValidator installs a process-wide validator used
// by protoc-gen-go-http generated handlers. It validates decoded protobuf
// request messages after HTTP binding and before business middleware runs.
func InstallProtovalidateRequestValidator(v protovalidate.Validator) {
	httpx.SetRequestValidator(httpx.RequestValidatorFunc(func(ctx context.Context, req any) error {
		if v == nil {
			return nil
		}
		msg, ok := req.(proto.Message)
		if !ok || msg == nil {
			return nil
		}
		if err := v.Validate(msg); err != nil {
			return errorx.BadRequest("REQUEST_VALIDATE_FAILED", "request validation failed", errorx.WithCause(err), errorx.WithMetadata("validation_error", err.Error()))
		}
		return nil
	}))
}
