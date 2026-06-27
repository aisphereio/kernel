package logx

import (
	"context"
	"runtime/debug"
	"time"
)

// Handler and Middleware mirror Kratos's middleware idea without importing
// Kratos. Kratos/gRPC/Connect adapters can translate their own transport data
// into this small interface.
type RPCHandler func(context.Context, any) (any, error)
type RPCMiddleware func(RPCHandler) RPCHandler

type RPCInfo struct {
	Side      string // server/client
	Protocol  string // grpc/rpc/http
	Component string
	Operation string
	Code      string
	Reason    string
	PeerAddr  string
}

type RPCInfoExtractor func(context.Context, any) RPCInfo

func ServerLogging(base Logger, extract RPCInfoExtractor) RPCMiddleware {
	return accessMiddleware(base, "server", extract)
}

func ClientLogging(base Logger, extract RPCInfoExtractor) RPCMiddleware {
	return accessMiddleware(base, "client", extract)
}

func accessMiddleware(base Logger, side string, extract RPCInfoExtractor) RPCMiddleware {
	if base == nil {
		base = Noop()
	}
	return func(next RPCHandler) RPCHandler {
		return func(ctx context.Context, req any) (reply any, err error) {
			start := time.Now()
			info := RPCInfo{Side: side, Protocol: "rpc"}
			if extract != nil {
				info = extract(ctx, req)
				if info.Side == "" {
					info.Side = side
				}
				if info.Protocol == "" {
					info.Protocol = "rpc"
				}
			}
			reply, err = next(ctx, req)
			logger := FromContextOr(ctx, base)
			LogAccess(logger, AccessEvent{
				Side:      info.Side,
				Protocol:  info.Protocol,
				Component: info.Component,
				Operation: info.Operation,
				Code:      info.Code,
				Reason:    info.Reason,
				Latency:   time.Since(start),
				Err:       err,
				Fields:    []Field{String("peer_addr", info.PeerAddr)},
			})
			return reply, err
		}
	}
}

type RPCPanicHandler func(ctx context.Context, req any, panicValue any) error

func RPCRecovery(base Logger, handler RPCPanicHandler) RPCMiddleware {
	if base == nil {
		base = Noop()
	}
	return func(next RPCHandler) RPCHandler {
		return func(ctx context.Context, req any) (reply any, err error) {
			defer func() {
				if rec := recover(); rec != nil {
					logger := FromContextOr(ctx, base)
					logger.Error("rpc panic recovered",
						Event("panic"),
						Any("panic", rec),
						String("stack", string(debug.Stack())),
					)
					if handler != nil {
						err = handler(ctx, req, rec)
					}
				}
			}()
			return next(ctx, req)
		}
	}
}
