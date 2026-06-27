// Package main demonstrates logx in a complete HTTP server with access log
// middleware, panic recovery, request-scoped fields, and dynamic level
// control.
//
// Run:
//
//	go run ./examples/logx-http
//
// Then in another terminal:
//
//	curl -i 'http://localhost:18080/?status=200'      # 200 → INFO access log
//	curl -i 'http://localhost:18080/?status=404'      # 404 → WARN access log
//	curl -i 'http://localhost:18080/?status=500'      # 500 → ERROR access log
//	curl -i 'http://localhost:18080/panic'            # recovered panic
//	curl -i 'http://localhost:18080/admin/log-level?level=debug'  # dynamic level
//	curl -i 'http://localhost:18080/?status=200'      # now debug logs visible
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aisphereio/kernel/logx"
)

func main() {
	// Bootstrap logger with prod-like config (JSON, no source)
	cfg := logx.DefaultConfig("prod")
	cfg.ServiceName = "demo-http"
	cfg.Version = "v1.0.0"
	cfg.AccessLog.Enabled = true
	cfg.AccessLog.SlowThreshold = 100 * time.Millisecond
	cfg.AccessLog.SkipPaths = []string{"/healthz"}
	logger, levelCtl, err := logx.New(cfg)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("server starting", logx.String("addr", ":18080"))

	mux := http.NewServeMux()

	// Health check (skipped from access log)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})

	// Main handler — demonstrates request-scoped fields + status control
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Inject request-scoped fields (in real code, extract from headers)
		ctx = logx.Inject(ctx, logger,
			logx.String("request_id", fmt.Sprintf("req_%d", time.Now().UnixNano())),
			logx.String("route", "/"),
			logx.String("method", r.Method),
		)

		logx.FromContext(ctx).Info("request started")

		statusStr := r.URL.Query().Get("status")
		status, _ := strconv.Atoi(statusStr)
		if status == 0 {
			status = 200
		}

		// Log business event
		logx.FromContext(ctx).Info("processing",
			logx.String("operation", "demo.handler"),
			logx.Int("target_status", status),
		)

		w.WriteHeader(status)
		_, _ = w.Write([]byte("ok\n"))

		logx.FromContext(ctx).Info("request finished",
			logx.Int("status", status),
		)
	})

	// Panic endpoint — recovered by logx.Recovery middleware
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("intentional panic for demo")
	})

	// Admin endpoint — dynamic log level control
	mux.Handle("/admin/log-level", logx.LevelHTTPHandler(levelCtl))

	// Compose middleware: Recovery → AccessLog → mux
	handler := logx.Recovery(logger)(
		logx.HTTPAccessLog(logger, cfg.AccessLog)(mux),
	)

	srv := &http.Server{
		Addr:              ":18080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		fmt.Println("demo-http listening on :18080")
		fmt.Println()
		fmt.Println("Try these in another terminal:")
		fmt.Println("  curl -i 'http://localhost:18080/?status=200'      # 200 → INFO access log")
		fmt.Println("  curl -i 'http://localhost:18080/?status=404'      # 404 → WARN access log")
		fmt.Println("  curl -i 'http://localhost:18080/?status=500'      # 500 → ERROR access log")
		fmt.Println("  curl -i 'http://localhost:18080/panic'            # recovered panic → ERROR")
		fmt.Println("  curl -i 'http://localhost:18080/healthz'          # skipped from access log")
		fmt.Println("  curl -i 'http://localhost:18080/admin/log-level?level=debug'  # dynamic level")
		fmt.Println()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nshutting down...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("server stopped")
}
