// Package main demonstrates configx Watch hot reload.
//
// Run:
//
//	go run ./examples/configx-watch
//
// The example writes a config file, loads it, registers a Watch callback on
// log.level, then rewrites the file to trigger a reload. The callback prints
// the new level.
//
// Expected output (timing may vary):
//
//	loaded config: log.level=info
//	watching log.level... (rewrite the file to trigger reload)
//	rewrote file: log.level=debug
//	reload detected: log.level=debug
//	done
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
)

func main() {
	dir, err := os.MkdirTemp("", "configx-watch-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "app.json")
	writeFile(path, `{"log":{"level":"info"}}`)

	cfg := configx.New(configx.WithSource(file.NewSource(path)))
	defer cfg.Close()

	if err := cfg.Load(); err != nil {
		panic(err)
	}

	level := configx.MustGet[string](cfg, "log.level")
	fmt.Printf("loaded config: log.level=%s\n", level)

	var saw atomic.Value
	saw.Store(level)
	if err := cfg.Watch("log.level", func(_ string, v configx.Value) {
		newLevel, _ := v.String()
		saw.Store(newLevel)
		fmt.Printf("reload detected: log.level=%s\n", newLevel)
	}); err != nil {
		panic(err)
	}

	fmt.Println("watching log.level... (rewrite the file to trigger reload)")

	// Rewrite the file after a short delay to give the watcher time to register.
	go func() {
		time.Sleep(200 * time.Millisecond)
		writeFile(path, `{"log":{"level":"debug"}}`)
		fmt.Println("rewrote file: log.level=debug")
	}()

	// Wait for the watcher to fire or timeout.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if saw.Load().(string) == "debug" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if saw.Load().(string) != "debug" {
		fmt.Println("WARNING: watcher did not fire within 5s (filesystem events may be delayed)")
	}

	fmt.Println("done")
}

func writeFile(path string, data string) {
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		panic(err)
	}
}
