package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
)

type AppConfig struct {
	App struct {
		Name string `json:"name"`
		Env  string `json:"env"`
	} `json:"app"`
	Server struct {
		Addr string `json:"addr"`
		Port int    `json:"port"`
	} `json:"server"`
}

func main() {
	dir, err := os.MkdirTemp("", "configx-basic-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "app.json")
	data := []byte(`{
  "app": { "name": "kernel", "env": "dev" },
  "server": { "addr": "0.0.0.0", "port": 8000 }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		panic(err)
	}

	cfg := configx.New(configx.WithSource(file.NewSource(path)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		panic(err)
	}

	var app AppConfig
	if err := cfg.Scan(&app); err != nil {
		panic(err)
	}

	fmt.Printf("%s/%s listens on %s:%d\n", app.App.Name, app.App.Env, app.Server.Addr, app.Server.Port)
}
