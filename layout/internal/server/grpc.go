package server

import (
	v1 "github.com/aisphereio/kernel-layout/api/todo/v1"
	"github.com/aisphereio/kernel-layout/internal/conf"
	"github.com/aisphereio/kernel-layout/internal/service"

	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c conf.ServerConfig, todo *service.TodoService) *kgrpc.Server {
	var opts []kgrpc.ServerOption
	if c.GRPC.Addr != "" {
		opts = append(opts, kgrpc.Address(c.GRPC.Addr))
	}
	if c.GRPC.Timeout > 0 {
		opts = append(opts, kgrpc.Timeout(c.GRPC.Timeout))
	}
	srv := kgrpc.NewServer(opts...)
	v1.RegisterTodoServiceServer(srv, todo)
	return srv
}
