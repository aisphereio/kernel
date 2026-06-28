package kernel

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

type metricsServer struct {
	addr    string
	path    string
	pprof   bool
	manager metricsx.Manager
	logger  logx.Logger
	server  *http.Server
}

func newMetricsServer(addr, path string, manager metricsx.Manager, logger logx.Logger, pprof bool) *metricsServer {
	if path == "" {
		path = "/metrics"
	}
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return &metricsServer{addr: addr, path: path, pprof: pprof, manager: manager, logger: logger.Named("kernel.metrics")}
}

func (s *metricsServer) Start(ctx context.Context) error {
	if s == nil || s.addr == "" {
		return nil
	}
	handler := metricsx.GetHandler(s.manager, metricsx.WithMetricsPath(s.path), metricsx.WithPprof(s.pprof))
	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.logger.Info("kernel metrics server starting", logx.String("addr", s.addr), logx.String("path", s.path), logx.Bool("pprof", s.pprof))
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		s.logger.Error("kernel metrics server failed", logx.String("addr", s.addr), logx.Err(err))
		return err
	}
	return nil
}

func (s *metricsServer) Stop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	s.logger.Info("kernel metrics server stopping", logx.String("addr", s.addr))
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("kernel metrics server stop failed", logx.String("addr", s.addr), logx.Err(err))
		return err
	}
	s.logger.Info("kernel metrics server stopped", logx.String("addr", s.addr))
	return nil
}
