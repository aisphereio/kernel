// Package dtmx provides a small kernel-native wrapper around DTM's HTTP Saga
// protocol. It deliberately avoids leaking dtm-labs SDK types into business
// code: services describe actions, compensations, and payloads; dtmx handles
// GID generation, request submission, logging, metrics, and errorx mapping.
package dtmx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

const (
	DefaultServer       = "http://127.0.0.1:36789/api/dtmsvr"
	DefaultBranchPrefix = "/internal/dtm"
)

var (
	ErrInvalidConfig = errorx.BadRequest(errorx.Code("DTMX_INVALID_CONFIG"), "dtmx config is invalid")
	ErrSubmitFailed  = errorx.Unavailable(errorx.Code("DTMX_SUBMIT_FAILED"), "failed to submit DTM transaction")
)

// Config configures the DTM HTTP client.
type Config struct {
	// Enabled controls whether the caller should use DTM orchestration.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Server is the DTM HTTP API base URL, usually
	// "http://127.0.0.1:36789/api/dtmsvr".
	Server string `json:"server" yaml:"server"`

	// ServiceBaseURL is the externally reachable base URL for branch action
	// callbacks, for example "http://127.0.0.1:18001". It is kept in dtmx so
	// business services do not hard-code their callback host.
	ServiceBaseURL string `json:"service_base_url" yaml:"service_base_url"`

	// BranchPrefix is prepended to branch paths when building callback URLs.
	// Default: /internal/dtm.
	BranchPrefix string `json:"branch_prefix" yaml:"branch_prefix"`

	// Timeout bounds calls from the application to the DTM server.
	Timeout time.Duration `json:"timeout_ns" yaml:"timeout_ns"`

	// WaitResult asks DTM to wait for the saga result before returning from
	// Submit. This is useful for request/response APIs such as skill upload.
	WaitResult bool `json:"wait_result" yaml:"wait_result"`

	// RequestTimeout / TimeoutToFail / RetryInterval are passed through to DTM
	// in seconds when greater than zero.
	RequestTimeout int64 `json:"request_timeout" yaml:"request_timeout"`
	TimeoutToFail  int64 `json:"timeout_to_fail" yaml:"timeout_to_fail"`
	RetryInterval  int64 `json:"retry_interval" yaml:"retry_interval"`

	Logger         logx.Logger      `json:"-" yaml:"-"`
	Metrics        metricsx.Manager `json:"-" yaml:"-"`
	MetricsEnabled bool             `json:"metrics_enabled" yaml:"metrics_enabled"`
}

func (c Config) normalized() Config {
	if strings.TrimSpace(c.Server) == "" {
		c.Server = DefaultServer
	}
	c.Server = strings.TrimRight(strings.TrimSpace(c.Server), "/")
	c.ServiceBaseURL = strings.TrimRight(strings.TrimSpace(c.ServiceBaseURL), "/")
	if strings.TrimSpace(c.BranchPrefix) == "" {
		c.BranchPrefix = DefaultBranchPrefix
	}
	if !strings.HasPrefix(c.BranchPrefix, "/") {
		c.BranchPrefix = "/" + c.BranchPrefix
	}
	c.BranchPrefix = "/" + strings.Trim(strings.TrimSpace(c.BranchPrefix), "/")
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.Logger == nil {
		c.Logger = logx.DefaultLogger().Named("dtmx")
	}
	if c.Metrics == nil {
		c.Metrics = metricsx.Noop()
	}
	return c
}

func (c Config) Validate() error {
	c = c.normalized()
	if c.Server == "" {
		return ErrInvalidConfig
	}
	if c.Enabled && c.ServiceBaseURL == "" {
		return errorx.From(ErrInvalidConfig, errorx.WithMessage("dtmx service_base_url is required when DTM is enabled"))
	}
	return nil
}

// Client is the kernel DTM client abstraction.
type Client interface {
	NewGID(ctx context.Context) (string, error)
	SubmitSaga(ctx context.Context, saga *Saga) error
	BranchURL(path string) string
	Config() Config
}

type HTTPClient struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) (*HTTPClient, error) {
	cfg = cfg.normalized()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	registerMetrics(cfg.Metrics)
	return &HTTPClient{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}, nil
}

func (c *HTTPClient) Config() Config { return c.cfg }

func (c *HTTPClient) BranchURL(path string) string {
	path = "/" + strings.TrimLeft(path, "/")
	return c.cfg.ServiceBaseURL + c.cfg.BranchPrefix + path
}

// NewGID returns a unique gid. DTM accepts caller-provided unique gid values;
// using UUID avoids adding a hard dependency on DTM's optional /newGid API.
func (c *HTTPClient) NewGID(ctx context.Context) (string, error) {
	return uuid.NewString(), nil
}

// Saga describes a DTM Saga transaction.
type Saga struct {
	GID            string              `json:"gid"`
	TransType      string              `json:"trans_type"`
	Steps          []map[string]string `json:"steps"`
	Payloads       []string            `json:"payloads"`
	WaitResult     bool                `json:"wait_result,omitempty"`
	RequestTimeout int64               `json:"request_timeout,omitempty"`
	TimeoutToFail  int64               `json:"timeout_to_fail,omitempty"`
	RetryInterval  int64               `json:"retry_interval,omitempty"`
	BranchHeaders  map[string]string   `json:"branch_headers,omitempty"`
}

func NewSaga(gid string) *Saga { return &Saga{GID: gid, TransType: "saga"} }

func (s *Saga) Add(action, compensate string, payload any) *Saga {
	if s.TransType == "" {
		s.TransType = "saga"
	}
	s.Steps = append(s.Steps, map[string]string{"action": action, "compensate": compensate})
	b, err := json.Marshal(payload)
	if err != nil {
		b = []byte(`{}`)
	}
	s.Payloads = append(s.Payloads, string(b))
	return s
}

func (c *HTTPClient) SubmitSaga(ctx context.Context, saga *Saga) error {
	if saga == nil || strings.TrimSpace(saga.GID) == "" || len(saga.Steps) == 0 || len(saga.Steps) != len(saga.Payloads) {
		return errorx.From(ErrInvalidConfig, errorx.WithMessage("invalid saga: gid and matched steps/payloads are required"))
	}
	if saga.TransType == "" {
		saga.TransType = "saga"
	}
	if saga.WaitResult == false {
		saga.WaitResult = c.cfg.WaitResult
	}
	if saga.RequestTimeout == 0 {
		saga.RequestTimeout = c.cfg.RequestTimeout
	}
	if saga.TimeoutToFail == 0 {
		saga.TimeoutToFail = c.cfg.TimeoutToFail
	}
	if saga.RetryInterval == 0 {
		saga.RetryInterval = c.cfg.RetryInterval
	}

	started := time.Now()
	body, err := json.Marshal(saga)
	if err != nil {
		return errorx.Wrap(err, errorx.Code("DTMX_ENCODE_FAILED"), errorx.WithMessage("failed to encode DTM saga"))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Server+"/submit", bytes.NewReader(body))
	if err != nil {
		return errorx.Wrap(err, errorx.Code("DTMX_REQUEST_FAILED"), errorx.WithMessage("failed to build DTM submit request"))
	}
	req.Header.Set("Content-Type", "application/json")

	c.cfg.Logger.WithContext(ctx).Info("dtm saga submit",
		logx.String("gid", saga.GID),
		logx.Int("steps", len(saga.Steps)),
		logx.Bool("wait_result", saga.WaitResult),
	)
	resp, err := c.client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		recordOperation(ctx, c.cfg, "submit_saga", "error", "DTMX_SUBMIT_FAILED", elapsed)
		c.cfg.Logger.WithContext(ctx).Warn("dtm saga submit failed",
			logx.String("gid", saga.GID), logx.Duration("elapsed", elapsed), logx.Err(err))
		return errorx.Wrap(err, errorx.Code("DTMX_SUBMIT_FAILED"), errorx.WithMessage("failed to submit DTM saga"))
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		recordOperation(ctx, c.cfg, "submit_saga", "error", fmt.Sprintf("HTTP_%d", resp.StatusCode), elapsed)
		c.cfg.Logger.WithContext(ctx).Warn("dtm saga submit rejected",
			logx.String("gid", saga.GID), logx.Int("status", resp.StatusCode), logx.Duration("elapsed", elapsed), logx.String("body", string(respBody)))
		return errorx.Wrap(ErrSubmitFailed, errorx.Code("DTMX_SUBMIT_FAILED"), errorx.WithMessage(fmt.Sprintf("DTM submit returned HTTP %d", resp.StatusCode)))
	}
	recordOperation(ctx, c.cfg, "submit_saga", "ok", "OK", elapsed)
	c.cfg.Logger.WithContext(ctx).Info("dtm saga submitted",
		logx.String("gid", saga.GID), logx.Duration("elapsed", elapsed))
	return nil
}
