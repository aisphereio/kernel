package grpc

import (
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/aisphereio/kernel/errorx"
)

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok && !errorx.IsKernelError(err) {
		return err
	}
	ke := errorx.From(err)
	st := status.New(toGRPCCode(ke.GRPCCode()), ke.Message())
	detail := &errdetails.ErrorInfo{
		Reason:   ke.Code().String(),
		Metadata: map[string]string{},
	}
	if ke.RequestID() != "" {
		detail.Metadata["request_id"] = ke.RequestID()
	}
	if ke.TraceID() != "" {
		detail.Metadata["trace_id"] = ke.TraceID()
	}
	for k, v := range ke.PublicMetadata() {
		detail.Metadata[k] = stringifyGRPCMetadataValue(v)
	}
	if len(detail.Metadata) > 0 || detail.Reason != "" {
		if withDetails, detailErr := st.WithDetails(detail); detailErr == nil {
			st = withDetails
		}
	}
	return st.Err()
}

func fromGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errorx.IsKernelError(err) {
		return err
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	code := codeFromGRPCStatus(st.Code())
	opts := []errorx.Option{
		errorx.WithMessage(st.Message()),
		errorx.WithHTTPStatus(fromGRPCCode(st.Code())),
		errorx.WithGRPCCode(st.Code().String()),
		errorx.WithCause(err),
	}
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			if info.Reason != "" {
				code = errorx.Code(info.Reason)
			}
			if info.Metadata != nil {
				public := make(map[string]any, len(info.Metadata))
				for k, v := range info.Metadata {
					switch k {
					case "request_id":
						opts = append(opts, errorx.WithRequestID(v))
					case "trace_id":
						opts = append(opts, errorx.WithTraceID(v))
					default:
						public[k] = v
					}
				}
				opts = append(opts, errorx.WithPublicMetadataMap(public))
			}
		}
	}
	return errorx.New(code, opts...)
}

func toGRPCCode(code string) codes.Code {
	switch code {
	case errorx.GRPCCodeOK:
		return codes.OK
	case errorx.GRPCCodeCanceled:
		return codes.Canceled
	case errorx.GRPCCodeInvalidArgument:
		return codes.InvalidArgument
	case errorx.GRPCCodeDeadlineExceeded:
		return codes.DeadlineExceeded
	case errorx.GRPCCodeNotFound:
		return codes.NotFound
	case errorx.GRPCCodeAlreadyExists:
		return codes.AlreadyExists
	case errorx.GRPCCodePermissionDenied:
		return codes.PermissionDenied
	case errorx.GRPCCodeResourceExhausted:
		return codes.ResourceExhausted
	case errorx.GRPCCodeUnavailable:
		return codes.Unavailable
	case errorx.GRPCCodeUnauthenticated:
		return codes.Unauthenticated
	case errorx.GRPCCodeInternal:
		return codes.Internal
	default:
		return codes.Internal
	}
}

func codeFromGRPCStatus(code codes.Code) errorx.Code {
	switch code {
	case codes.OK:
		return errorx.CodeOK
	case codes.Canceled:
		return errorx.CodeClientClosedRequest
	case codes.InvalidArgument:
		return errorx.CodeBadRequest
	case codes.DeadlineExceeded:
		return errorx.CodeTimeout
	case codes.NotFound:
		return errorx.CodeNotFound
	case codes.AlreadyExists:
		return errorx.CodeConflict
	case codes.PermissionDenied:
		return errorx.CodeForbidden
	case codes.ResourceExhausted:
		return errorx.CodeTooManyRequests
	case codes.Unavailable:
		return errorx.CodeUnavailable
	case codes.Unauthenticated:
		return errorx.CodeUnauthorized
	default:
		return errorx.CodeInternal
	}
}

func fromGRPCCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return errorx.HTTPStatusOK
	case codes.Canceled:
		return errorx.HTTPStatusClientClosedRequest
	case codes.InvalidArgument:
		return errorx.HTTPStatusBadRequest
	case codes.DeadlineExceeded:
		return errorx.HTTPStatusGatewayTimeout
	case codes.NotFound:
		return errorx.HTTPStatusNotFound
	case codes.AlreadyExists:
		return errorx.HTTPStatusConflict
	case codes.PermissionDenied:
		return errorx.HTTPStatusForbidden
	case codes.ResourceExhausted:
		return errorx.HTTPStatusTooManyRequests
	case codes.Unavailable:
		return errorx.HTTPStatusServiceUnavailable
	case codes.Unauthenticated:
		return errorx.HTTPStatusUnauthorized
	default:
		return errorx.HTTPStatusInternalServerError
	}
}

func stringifyGRPCMetadataValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case error:
		return x.Error()
	default:
		return fmt.Sprint(v)
	}
}
