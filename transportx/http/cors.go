package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CORSConfig configures the built-in HTTP CORS filter.
type CORSConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers" yaml:"allowed_headers"`
	ExposedHeaders []string `json:"exposed_headers" yaml:"exposed_headers"`

	AllowCredentials bool          `json:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           time.Duration `json:"max_age_ns" yaml:"max_age_ns"`
}

// CORS installs a CORS filter. Unlike Filter(...), it appends to the existing
// filter chain so callers can combine it safely with authn/custom filters.
func CORS(cfg CORSConfig) ServerOption {
	return func(s *Server) {
		if !cfg.Enabled {
			return
		}
		s.filters = append(s.filters, CORSFilter(cfg))
	}
}

// CORSFilter returns a standard net/http middleware implementing browser CORS
// preflight and response headers. It is also exported so apps can use it with
// the generic Filter option when needed.
func CORSFilter(cfg CORSConfig) FilterFunc {
	cfg = normalizeCORSConfig(cfg)
	allowedOrigins := stringSet(cfg.AllowedOrigins)
	allowAnyOrigin := allowedOrigins["*"]
	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := ""
	if cfg.MaxAge > 0 {
		maxAge = strconv.FormatInt(int64(cfg.MaxAge/time.Second), 10)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !allowAnyOrigin && !allowedOrigins[origin] {
				next.ServeHTTP(w, r)
				return
			}

			setCORSHeaders(w, origin, allowAnyOrigin, cfg.AllowCredentials, allowedMethods, allowedHeaders, exposedHeaders, maxAge)
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func normalizeCORSConfig(cfg CORSConfig) CORSConfig {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Authorization", "Content-Type", "X-Request-ID", "X-Trace-ID", "Traceparent", "Tracestate"}
	}
	return cfg
}

func setCORSHeaders(w http.ResponseWriter, origin string, allowAnyOrigin, allowCredentials bool, methods, headers, exposed, maxAge string) {
	w.Header().Add("Vary", "Origin")
	w.Header().Add("Vary", "Access-Control-Request-Method")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	if allowAnyOrigin && !allowCredentials {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	if allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if methods != "" {
		w.Header().Set("Access-Control-Allow-Methods", methods)
	}
	if headers != "" {
		w.Header().Set("Access-Control-Allow-Headers", headers)
	}
	if exposed != "" {
		w.Header().Set("Access-Control-Expose-Headers", exposed)
	}
	if maxAge != "" {
		w.Header().Set("Access-Control-Max-Age", maxAge)
	}
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}
