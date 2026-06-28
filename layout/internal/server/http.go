package server

import (
	"encoding/json"
	"net/http"
	"time"

	v1 "github.com/aisphereio/kernel-layout/api/todo/v1"
	"github.com/aisphereio/kernel-layout/internal/conf"
	"github.com/aisphereio/kernel-layout/internal/data"
	"github.com/aisphereio/kernel-layout/internal/service"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

func NewHTTPServer(cfg conf.ServerConfig, resources *data.Resources, todo *service.TodoService) *khttp.Server {
	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = "0.0.0.0:8000"
	}
	timeout := cfg.HTTP.Timeout
	if timeout <= 0 {
		timeout = time.Second
	}
	srv := khttp.NewServer(khttp.Address(addr), khttp.Timeout(timeout))
	v1.RegisterTodoServiceHTTPServer(srv, todo)
	srv.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	srv.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if resources == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	return srv
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
