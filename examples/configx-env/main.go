// Package main demonstrates configx layered file + env override.
//
// The env source produces flat key/value entries. To override a nested config
// path like app.env, use a placeholder in the base file that references the
// env-injected top-level key.
//
// Run:
//
//	go run ./examples/configx-env
//
// Expected output:
//
//	loaded config:
//	app.name=kernel
//	app.env=prod       (resolved from ${APP_ENV} via KERNEL_APP_ENV)
//	server.port=9000   (resolved from ${SERVER_PORT} via KERNEL_SERVER_PORT)
//	server.addr=0.0.0.0 (from base file)
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/env"
	"github.com/aisphereio/kernel/configx/file"
)

func main() {
	dir, err := os.MkdirTemp("", "configx-env-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	// Write a base config file with placeholders that reference env-injected
	// top-level keys.
	base := []byte(`{
  "APP_ENV": "dev",
  "SERVER_PORT": 8000,
  "app": { "name": "kernel", "env": "${APP_ENV}" },
  "server": { "addr": "0.0.0.0", "port": "${SERVER_PORT}" }
}`)
	if err := os.WriteFile(filepath.Join(dir, "base.json"), base, 0o600); err != nil {
		panic(err)
	}

	// Simulate environment overrides injected at deploy time.
	os.Setenv("KERNEL_APP_ENV", "prod")
	os.Setenv("KERNEL_SERVER_PORT", "9000")
	defer os.Unsetenv("KERNEL_APP_ENV")
	defer os.Unsetenv("KERNEL_SERVER_PORT")

	cfg := configx.New(configx.WithSource(
		file.NewSource(filepath.Join(dir, "base.json")),
		env.NewSource("KERNEL_"),
	), configx.WithResolveActualTypes(true))
	defer cfg.Close()

	if err := cfg.Load(); err != nil {
		panic(err)
	}

	fmt.Println("loaded config:")
	fmt.Printf("app.name=%s\n", configx.MustGet[string](cfg, "app.name"))
	fmt.Printf("app.env=%s       (resolved from ${APP_ENV} via KERNEL_APP_ENV)\n", configx.MustGet[string](cfg, "app.env"))
	fmt.Printf("server.port=%d   (resolved from ${SERVER_PORT} via KERNEL_SERVER_PORT)\n", configx.MustGet[int](cfg, "server.port"))
	fmt.Printf("server.addr=%s (from base file)\n", configx.MustGet[string](cfg, "server.addr"))
}
