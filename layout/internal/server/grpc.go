package server

import (
	v1 "github.com/aisphereio/kernel-layout/api/todo/v1"
	"github.com/aisphereio/kernel-layout/internal/conf"
	"github.com/aisphereio/kernel-layout/internal/data"
	"github.com/aisphereio/kernel-layout/internal/service"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
)

func NewGRPCServer(c conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, resources *data.Resources, todo *service.TodoService, securityCfg conf.SecurityConfig) *kgrpc.Server {
	var opts []kgrpc.ServerOption
	if c.GRPC.Addr != "" {
		opts = append(opts, kgrpc.Address(c.GRPC.Addr))
	}
	if c.GRPC.Timeout > 0 {
		opts = append(opts, kgrpc.Timeout(c.GRPC.Timeout))
	}
	opts = append(opts,
		kgrpc.Logger(logger.Named("transport.grpc")),
		kgrpc.AccessLog(logCfg.AccessLog),
	)
	if metricsCfg.Enabled {
		opts = append(opts, kgrpc.Metrics(metrics))
	}
	if m := todoServerMiddlewares(resources, securityCfg); len(m) > 0 {
		opts = append(opts, kgrpc.Middleware(m...))
	}
	srv := kgrpc.NewServer(opts...)
	v1.RegisterTodoServiceServer(srv, todo)
	return srv
}
