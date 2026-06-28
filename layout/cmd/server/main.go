package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	kernel "github.com/aisphereio/kernel"
	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/logx"

	"github.com/aisphereio/kernel-layout/internal/biz"
	"github.com/aisphereio/kernel-layout/internal/conf"
	"github.com/aisphereio/kernel-layout/internal/data"
	"github.com/aisphereio/kernel-layout/internal/server"
	"github.com/aisphereio/kernel-layout/internal/service"
)

var (
	Name     = "app"
	Version  = "dev"
	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "configs", "config path, eg: -conf configs")
}

func main() {
	flag.Parse()

	cfg := configx.New(configx.WithSource(file.NewSource(flagconf)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := cfg.Scan(&bc); err != nil {
		panic(err)
	}
	applyBuildInfo(&bc)

	logger := newLogger(bc.Log)
	resources, cleanup, err := data.NewResources(context.Background(), bc)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	dataStore := data.NewData(resources)
	todoRepo := data.NewTodoRepo(dataStore)
	todoUsecase := biz.NewTodoUsecase(todoRepo)
	todoService := service.NewTodoService(todoUsecase)
	httpServer := server.NewHTTPServer(bc.Server, resources, todoService)
	grpcServer := server.NewGRPCServer(bc.Server, todoService)
	app := kernel.New(
		kernel.Name(bc.Service.Name),
		kernel.Version(bc.Service.Version),
		kernel.Logger(logger),
		kernel.Server(httpServer, grpcServer),
		kernel.StopTimeout(10*time.Second),
	)
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func applyBuildInfo(bc *conf.Bootstrap) {
	if bc.Service.Name == "" {
		bc.Service.Name = Name
	}
	if bc.Service.Version == "" {
		bc.Service.Version = Version
	}
	if bc.Log.ServiceName == "" {
		bc.Log.ServiceName = bc.Service.Name
	}
	if bc.Log.Version == "" {
		bc.Log.Version = bc.Service.Version
	}
}

func newLogger(cfg logx.Config) *slog.Logger {
	if cfg.Output == "" {
		cfg.Output = string(logx.OutputStdout)
	}
	writer := os.Stdout
	if cfg.Output == string(logx.OutputStderr) {
		writer = os.Stderr
	}
	return slog.New(logx.NewHandler(
		logx.WithWriter(writer),
		logx.WithFormat(cfg.Format),
		logx.WithLevel(logx.ParseLevel(cfg.Level)),
		logx.WithAddSource(cfg.AddSource),
	))
}
