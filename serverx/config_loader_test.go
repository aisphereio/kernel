package serverx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigFileControlsHTTPAndGRPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.yaml")
	data := []byte(`app:
  name: skill-service
  version: test

deployment:
  replicas: 1

server:
  http:
    enabled: true
    address: ":18080"
    timeout: "3s"
  grpc:
    enabled: true
    address: ":19090"
    timeout: "5s"

system_routes:
  enabled: true
  healthz: true
  readyz: true
  version: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "skill-service" || cfg.Version != "test" {
		t.Fatalf("unexpected app config: %#v", cfg)
	}
	if !cfg.HTTP.Enabled || cfg.HTTP.Address != ":18080" || cfg.HTTP.Timeout != 3*time.Second {
		t.Fatalf("unexpected http config: %#v", cfg.HTTP)
	}
	if !cfg.GRPC.Enabled || cfg.GRPC.Address != ":19090" || cfg.GRPC.Timeout != 5*time.Second {
		t.Fatalf("unexpected grpc config: %#v", cfg.GRPC)
	}
}

func TestNewFromConfigCanEnableGRPCOnly(t *testing.T) {
	var fc FileConfig
	fc.App.Name = "iam-service"
	fc.Server.GRPC.Enabled = true
	fc.Server.GRPC.Address = ":0"
	fc.Server.GRPC.Timeout = "1s"
	cfg, err := DecodeFileConfig(fc)
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if app.HTTP() != nil {
		t.Fatalf("http should be disabled")
	}
	if app.GRPC() == nil {
		t.Fatalf("grpc should be enabled")
	}
}
