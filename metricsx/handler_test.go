package metricsx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aisphereio/kernel/metricsx"
)

func TestGetHandlerCustomPathAndPprofDisabled(t *testing.T) {
	handler := metricsx.GetHandler(metricsx.Noop(), metricsx.WithMetricsPath("/internal/metrics"), metricsx.WithPprof(false))

	req := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("pprof status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetHandlerDefaultPprofEnabled(t *testing.T) {
	handler := metricsx.GetHandler(metricsx.Noop())
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pprof status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetMetricsHandler(t *testing.T) {
	handler := metricsx.GetMetricsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics-only handler status = %d, want %d", rec.Code, http.StatusOK)
	}
}
