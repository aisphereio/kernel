// Package main demonstrates a complete HTTP handler that returns errorx errors.
//
// Run:
//
//	go run ./examples/errorx-http
//
// Then in another terminal:
//
//	curl -i http://localhost:18080/skills/skill_001
//	curl -i -X POST http://localhost:18080/skills -d '{"name":""}'
//	curl -i http://localhost:18080/skills/forbidden
//	curl -i http://localhost:18080/skills/boom
//
// Each endpoint returns a different errorx error, illustrating how the same
// handler pattern produces consistent HTTP responses with stable error_code.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aisphereio/kernel/errorx"
)

// ============================================================================
// Domain error codes (declare once per module)
// ============================================================================

const (
	ErrSkillNotFound      errorx.Code = "AIHUB_SKILL_NOT_FOUND"
	ErrSkillNameRequired  errorx.Code = "AIHUB_SKILL_NAME_REQUIRED"
	ErrSkillCreateDenied  errorx.Code = "AIHUB_SKILL_CREATE_DENIED"
	ErrSkillQueryFailed   errorx.Code = "AIHUB_SKILL_QUERY_FAILED"
	ErrSkillAlreadyExists errorx.Code = "AIHUB_SKILL_ALREADY_EXISTS"
)

// ============================================================================
// Fake repository (in real code this would be backed by Postgres)
// ============================================================================

type Skill struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var skills = map[string]*Skill{
	"skill_001": {ID: "skill_001", Name: "demo"},
}

func findSkill(ctx context.Context, id string) (*Skill, error) {
	// Simulate not-found via sql.ErrNoRows so we can show the standard pattern.
	if id == "missing" {
		return nil, sql.ErrNoRows
	}
	if id == "boom" {
		// Simulate a database failure.
		return nil, errors.New("pq: connection refused")
	}
	s, ok := skills[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return s, nil
}

// ============================================================================
// HTTP handler — returns errorx errors, never raw errors
// ============================================================================

type SkillHandler struct{}

func (h *SkillHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErrorx(w, errorx.BadRequest("AIHUB_SKILL_ID_REQUIRED", "skill id is required",
			errorx.WithPublicMetadata("field", "id"),
		))
		return
	}

	skill, err := findSkill(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErrorx(w, errorx.NotFound(ErrSkillNotFound, "skill not found",
			errorx.WithCause(err),
			errorx.WithMetadata("skill_id", id),
			errorx.WithPublicMetadata("resource", "skill"),
		))
		return
	}
	if err != nil {
		writeErrorx(w, errorx.Wrap(err, ErrSkillQueryFailed,
			errorx.WithMessage("query skill failed"),
			errorx.WithRetryable(true),
			errorx.WithMetadata("skill_id", id),
		))
		return
	}

	writeJSON(w, http.StatusOK, skill)
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorx(w, errorx.BadRequest("AIHUB_SKILL_BODY_INVALID", "invalid request body",
			errorx.WithCause(err),
		))
		return
	}

	if req.Name == "" {
		writeErrorx(w, errorx.BadRequest(ErrSkillNameRequired, "skill name is required",
			errorx.WithPublicMetadata("field", "name"),
		))
		return
	}

	// Simulate "no permission" for a specific name to show the Forbidden pattern.
	if req.Name == "forbidden" {
		writeErrorx(w, errorx.Forbidden(ErrSkillCreateDenied, "no permission to create this skill",
			errorx.WithPublicMetadata("name", req.Name),
		))
		return
	}

	// Simulate "already exists" to show the Conflict pattern.
	if _, exists := skills[req.Name]; exists {
		writeErrorx(w, errorx.Conflict(ErrSkillAlreadyExists, "skill already exists",
			errorx.WithPublicMetadata("name", req.Name),
		))
		return
	}

	skill := &Skill{ID: req.Name, Name: req.Name}
	skills[req.Name] = skill
	writeJSON(w, http.StatusCreated, skill)
}

// ============================================================================
// writeErrorx: the single place that converts errorx → HTTP response.
// In real code this lives in kernel httpx middleware; here we inline it for
// the example.
// ============================================================================

type errorResponse struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func writeErrorx(w http.ResponseWriter, err error) {
	resp := errorResponse{
		Code:      errorx.CodeOf(err).String(),
		Message:   errorx.MessageOf(err),
		RequestID: errorx.RequestIDOf(err),
		TraceID:   errorx.TraceIDOf(err),
		Metadata:  errorx.PublicMetadataOf(err), // only public metadata, never raw Metadata
	}

	// Log the full error with internal metadata for debugging.
	slog.Error("request failed",
		slog.String("error_code", resp.Code),
		slog.Int("http_status", errorx.HTTPStatusOf(err)),
		slog.Bool("retryable", errorx.RetryableOf(err)),
		slog.String("category", errorx.CategoryOf(err).String()),
		slog.Any("metadata", errorx.SafeMetadataOf(err)), // redacted
		slog.Any("error", err),
	)

	writeJSON(w, errorx.HTTPStatusOf(err), resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ============================================================================
// Main
// ============================================================================

func main() {
	mux := http.NewServeMux()
	h := &SkillHandler{}

	// GET /skills?id=skill_001
	mux.HandleFunc("/skills", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.Get(w, r)
		case http.MethodPost:
			h.Create(w, r)
		default:
			writeErrorx(w, errorx.BadRequest("HTTP_METHOD_NOT_ALLOWED", "method not allowed"))
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	addr := ":18080"
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		fmt.Printf("errorx-http example listening on %s\n", addr)
		fmt.Println()
		fmt.Println("Try these in another terminal:")
		fmt.Println("  curl -i 'http://localhost:18080/skills?id=skill_001'      # 200")
		fmt.Println("  curl -i 'http://localhost:18080/skills?id=missing'        # 404 AIHUB_SKILL_NOT_FOUND")
		fmt.Println("  curl -i 'http://localhost:18080/skills?id=boom'           # 500 AIHUB_SKILL_QUERY_FAILED")
		fmt.Println("  curl -i 'http://localhost:18080/skills?id='               # 400 AIHUB_SKILL_ID_REQUIRED")
		fmt.Println("  curl -i -X POST http://localhost:18080/skills -d '{\"name\":\"\"}'         # 400 AIHUB_SKILL_NAME_REQUIRED")
		fmt.Println("  curl -i -X POST http://localhost:18080/skills -d '{\"name\":\"forbidden\"}' # 403 AIHUB_SKILL_CREATE_DENIED")
		fmt.Println("  curl -i -X POST http://localhost:18080/skills -d '{\"name\":\"demo\"}'      # 409 AIHUB_SKILL_ALREADY_EXISTS")
		fmt.Println()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nshutting down...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
}
