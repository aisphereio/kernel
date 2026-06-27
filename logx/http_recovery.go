package logx

import (
	"context"
	"net/http"
	"runtime/debug"
)

type PanicHandler func(ctx context.Context, panicValue any) (status int, body any)

type RecoveryOption func(*recoveryOptions)

type recoveryOptions struct {
	handler PanicHandler
}

func WithPanicHandler(handler PanicHandler) RecoveryOption {
	return func(o *recoveryOptions) { o.handler = handler }
}

func Recovery(base Logger, opts ...RecoveryOption) func(http.Handler) http.Handler {
	if base == nil {
		base = Noop()
	}
	options := recoveryOptions{
		handler: func(context.Context, any) (int, any) {
			return http.StatusInternalServerError, map[string]any{
				"code":    "INTERNAL_SERVER_ERROR",
				"message": "internal server error",
			}
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					ctx := r.Context()
					logger := FromContextOr(ctx, base)
					logger.Error("panic recovered",
						Event("panic"),
						Any("panic", rec),
						String("stack", string(debug.Stack())),
						String("method", r.Method),
						String("path", r.URL.Path),
					)
					status, body := options.handler(ctx, rec)
					if status == 0 {
						status = http.StatusInternalServerError
					}
					writeJSON(w, status, body)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
