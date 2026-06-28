package kernel

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/registry"
	transport "github.com/aisphereio/kernel/transportx"
)

// AppInfo is application context value.
type AppInfo interface {
	ID() string
	Name() string
	Version() string
	Metadata() map[string]string
	Endpoint() []string
}

// App is an application components lifecycle manager.
type App struct {
	opts     options
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	instance *registry.ServiceInstance
}

// New create an application lifecycle manager.
func New(opts ...Option) *App {
	o := options{
		ctx:              context.Background(),
		sigs:             []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
		registrarTimeout: 10 * time.Second,
	}
	if id, err := uuid.NewUUID(); err == nil {
		o.id = id.String()
	}
	for _, opt := range opts {
		opt(&o)
	}
	logger := configureDefaultLogger(&o)
	if o.logxLogger == nil {
		o.logxLogger = logger
	}
	o.metrics = configureDefaultMetrics(&o, logger)
	o.dtm = configureDefaultDTM(&o)
	if o.prometheusMetrics && o.metricsAddr != "" {
		o.servers = append(o.servers, newMetricsServer(o.metricsAddr, o.metricsPath, o.metrics, logger, o.metricsPprof))
	}
	ctx, cancel := context.WithCancel(o.ctx)
	return &App{
		ctx:    ctx,
		cancel: cancel,
		opts:   o,
	}
}

// ID returns app instance id.
func (a *App) ID() string { return a.opts.id }

// Name returns service name.
func (a *App) Name() string { return a.opts.name }

// Version returns app version.
func (a *App) Version() string { return a.opts.version }

// Metadata returns service metadata.
func (a *App) Metadata() map[string]string { return a.opts.metadata }

// Endpoint returns endpoints.
func (a *App) Endpoint() []string {
	if a.instance != nil {
		return a.instance.Endpoints
	}
	return nil
}

// Run executes all OnStart hooks registered with the application's Lifecycle.
func (a *App) Run() error {
	logger := a.logger()
	logger.Info("kernel app starting", logx.Int("server_count", len(a.opts.servers)))
	instance, err := a.buildInstance()
	if err != nil {
		logger.Error("kernel app build instance failed", logx.Err(err))
		return err
	}
	a.mu.Lock()
	a.instance = instance
	a.mu.Unlock()
	sctx := injectAppObservability(NewContext(a.ctx, a), logger, a.metrics(), a.dtm())
	eg, ctx := errgroup.WithContext(sctx)
	wg := sync.WaitGroup{}

	for _, fn := range a.opts.beforeStart {
		if err = fn(sctx); err != nil {
			logger.Error("kernel before-start hook failed", logx.Err(err))
			return err
		}
	}
	octx := injectAppObservability(NewContext(a.opts.ctx, a), logger, a.metrics(), a.dtm())
	for _, srv := range a.opts.servers {
		server := srv
		eg.Go(func() error {
			<-ctx.Done() // wait for stop signal
			stopCtx := context.WithoutCancel(octx)
			if a.opts.stopTimeout > 0 {
				var cancel context.CancelFunc
				stopCtx, cancel = context.WithTimeout(stopCtx, a.opts.stopTimeout)
				defer cancel()
			}
			if err := server.Stop(stopCtx); err != nil {
				logger.Error("kernel server stop failed", logx.Err(err))
				return err
			}
			logger.Info("kernel server stopped")
			return nil
		})
		wg.Add(1)
		eg.Go(func() error {
			wg.Done() // here is to ensure server start has begun running before register, so defer is not needed
			if err := server.Start(octx); err != nil {
				logger.Error("kernel server start failed", logx.Err(err))
				return err
			}
			return nil
		})
	}
	wg.Wait()
	if a.opts.registrar != nil {
		rctx, rcancel := context.WithTimeout(ctx, a.opts.registrarTimeout)
		defer rcancel()
		if err = a.opts.registrar.Register(rctx, instance); err != nil {
			logger.Error("kernel service registration failed", logx.Err(err))
			return err
		}
		logger.Info("kernel service registered", logx.Any("endpoints", instance.Endpoints))
	}
	for _, fn := range a.opts.afterStart {
		if err = fn(sctx); err != nil {
			logger.Error("kernel after-start hook failed", logx.Err(err))
			return err
		}
	}
	logger.Info("kernel app started", logx.Any("endpoints", instance.Endpoints))

	c := make(chan os.Signal, 1)
	signal.Notify(c, a.opts.sigs...)
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case <-c:
			return a.Stop()
		}
	})
	if err = eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("kernel app stopped with error", logx.Err(err))
		return err
	}
	err = nil
	for _, fn := range a.opts.afterStop {
		err = fn(sctx)
		if err != nil {
			logger.Error("kernel after-stop hook failed", logx.Err(err))
		}
	}
	logger.Info("kernel app stopped")
	return err
}

// Stop gracefully stops the application.
func (a *App) Stop() (err error) {
	logger := a.logger()
	logger.Info("kernel app stopping")
	sctx := injectAppObservability(NewContext(a.ctx, a), logger, a.metrics(), a.dtm())
	for _, fn := range a.opts.beforeStop {
		err = fn(sctx)
		if err != nil {
			logger.Error("kernel before-stop hook failed", logx.Err(err))
		}
	}

	a.mu.Lock()
	instance := a.instance
	a.mu.Unlock()
	if a.opts.registrar != nil && instance != nil {
		ctx, cancel := context.WithTimeout(injectAppObservability(NewContext(a.ctx, a), logger, a.metrics(), a.dtm()), a.opts.registrarTimeout)
		defer cancel()
		if err = a.opts.registrar.Deregister(ctx, instance); err != nil {
			logger.Error("kernel service deregistration failed", logx.Err(err))
			return err
		}
		logger.Info("kernel service deregistered", logx.Any("endpoints", instance.Endpoints))
	}
	if a.cancel != nil {
		a.cancel()
	}
	return err
}

func (a *App) buildInstance() (*registry.ServiceInstance, error) {
	endpoints := make([]string, 0, len(a.opts.endpoints))
	for _, e := range a.opts.endpoints {
		endpoints = append(endpoints, e.String())
	}
	if len(endpoints) == 0 {
		for _, srv := range a.opts.servers {
			if r, ok := srv.(transport.Endpointer); ok {
				e, err := r.Endpoint()
				if err != nil {
					return nil, err
				}
				endpoints = append(endpoints, e.String())
			}
		}
	}
	return &registry.ServiceInstance{
		ID:        a.opts.id,
		Name:      a.opts.name,
		Version:   a.opts.version,
		Metadata:  a.opts.metadata,
		Endpoints: endpoints,
	}, nil
}

type appKey struct{}

// NewContext returns a new Context that carries value.
func NewContext(ctx context.Context, s AppInfo) context.Context {
	return context.WithValue(ctx, appKey{}, s)
}

// FromContext returns the Transport value stored in ctx, if any.
func FromContext(ctx context.Context) (s AppInfo, ok bool) {
	s, ok = ctx.Value(appKey{}).(AppInfo)
	return
}
