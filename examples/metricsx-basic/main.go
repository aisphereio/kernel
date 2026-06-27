// Package main demonstrates metricsx basic usage.
//
// Run:
//
//	go run ./examples/metricsx-basic
//
// Then in another terminal:
//
//	curl http://localhost:9001/metrics
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

func main() {
	// 1. Bootstrap Prometheus-backed Manager
	m := metricsx.NewPrometheusManager("demo-app", "v1.0.0", logx.Noop())

	// 2. Register system metrics (goroutines, memory, GC)
	metricsx.RegisterSystemMetrics(m)

	// 3. Register business metrics
	m.NewCounter("skill_create_total", "Total skills created")
	m.NewCounter("errors_total", "Total errors by code")
	m.NewHistogram("request_seconds", "Request latency", metricsx.DefaultBuckets...)
	m.NewGauge("active_sessions", "Active sessions")
	m.NewUpDownCounter("queue_size", "Current queue size")

	// 4. Expose /metrics + /debug/pprof/*
	go func() {
		http.Handle("/metrics", metricsx.GetHandler(m))
		fmt.Println("metrics server listening on :9001")
		fmt.Println("try: curl http://localhost:9001/metrics")
		if err := http.ListenAndServe(":9001", nil); err != nil {
			fmt.Println("metrics server error:", err)
		}
	}()

	// 5. Simulate business activity
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	sessionCount := 0
	tickCount := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nshutting down...")
			return
		case <-ticker.C:
			tickCount++
			// Simulate request
			start := time.Now()
			m.IncrementCounter(ctx, "skill_create_total",
				"tenant", "t_acme",
				"visibility", "public",
			)
			m.RecordHistogram(ctx, "request_seconds", time.Since(start).Seconds(),
				"operation", "create_skill",
			)

			// Simulate session changes
			sessionCount++
			m.DeltaUpDownCounter(ctx, "queue_size", 1, "queue", "publish")
			m.SetGauge("active_sessions", float64(sessionCount), "tenant", "t_acme")

			// Every 5th tick, simulate an error
			if tickCount%5 == 0 {
				m.IncrementCounter(ctx, "errors_total",
					"error_code", "AIHUB_SKILL_NOT_FOUND",
					"http_status", "404",
				)
			}

			if tickCount%4 == 0 {
				fmt.Printf("tick %d: created skills, active_sessions=%d\n", tickCount, sessionCount)
			}
		}
	}
}
