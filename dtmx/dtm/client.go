package dtm

import (
	"context"
	"fmt"
	"time"

	"github.com/aisphereio/kernel/dtmx"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"

	"github.com/dtm-labs/client/dtmcli"
)

const driverName = dtmx.DriverDTM

func init() {
	dtmx.RegisterDriver(driverName, open)
}

type client struct {
	cfg    dtmx.Config
	logger logx.Logger
}

func open(cfg dtmx.Config) (dtmx.Manager, error) {
	if !cfg.Enabled {
		return dtmx.Disabled(), nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &client{cfg: cfg, logger: cfg.Logger.Named("dtmx.dtm")}, nil
}

func (c *client) Enabled() bool { return c != nil && c.cfg.Enabled }

func (c *client) DriverName() string { return driverName }

func (c *client) Close() error { return nil }

func (c *client) BranchURL(branchPath string) string {
	if c == nil {
		return ""
	}
	return dtmxBranchURL(c.cfg, branchPath)
}

func (c *client) branchHeaders(saga dtmx.Saga) map[string]string {
	headers := make(map[string]string, len(saga.Options.BranchHeaders)+1)
	if c != nil && c.cfg.BranchSecret != "" {
		headers[dtmx.BranchAuthHeader] = c.cfg.BranchSecret
	}
	for k, v := range saga.Options.BranchHeaders {
		if k != "" {
			headers[k] = v
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func (c *client) NewGID(ctx context.Context) (gid string, err error) {
	if c == nil || !c.Enabled() {
		return "", dtmxDisabledError()
	}
	if err := ctx.Err(); err != nil {
		return "", wrapContextError(err, "dtm create gid canceled")
	}
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		status := "ok"
		code := errorx.CodeOK.String()
		if err != nil {
			status = "error"
			code = errorx.CodeOf(err).String()
			c.logger.WithContext(ctx).Error("dtm create gid failed", logx.Duration("elapsed", elapsed), logx.Err(err))
		} else {
			c.logger.WithContext(ctx).Debug("dtm gid created", logx.String("gid", gid), logx.Duration("elapsed", elapsed))
		}
		record(ctx, c.cfg, "new_gid", status, code, elapsed)
	}()
	defer func() {
		if r := recover(); r != nil {
			err = errorx.Unavailable(dtmx.CodeNewGIDFailed, "dtm create gid failed", errorx.WithMetadata("panic", fmt.Sprint(r)), errorx.WithRetryable(true))
			gid = ""
		}
	}()
	return dtmcli.MustGenGid(c.cfg.Server), nil
}

func (c *client) SubmitSaga(ctx context.Context, saga dtmx.Saga) (dtmx.TransactionResult, error) {
	started := time.Now()
	result := dtmx.TransactionResult{GID: saga.GID, Protocol: dtmx.ProtocolHTTP, Pattern: "saga", StartedAt: started}
	if c == nil || !c.Enabled() {
		return result, dtmxDisabledError()
	}
	if saga.GID == "" {
		gid, err := c.NewGID(ctx)
		if err != nil {
			return result, err
		}
		saga.GID = gid
		result.GID = gid
	}
	if err := saga.Validate(); err != nil {
		wrapped := errorx.BadRequest(dtmx.CodeInvalidSaga, "invalid saga", errorx.WithCause(err))
		c.logger.WithContext(ctx).Warn("dtm saga invalid", logx.String("gid", saga.GID), logx.String("saga", saga.Name), logx.Err(wrapped))
		record(ctx, c.cfg, "submit_saga", "error", errorx.CodeOf(wrapped).String(), time.Since(started))
		return result, wrapped
	}

	waitResult := c.cfg.WaitResult
	if saga.Options.WaitResult != nil {
		waitResult = *saga.Options.WaitResult
	}
	timeout := c.cfg.Timeout
	if saga.Options.Timeout > 0 {
		timeout = saga.Options.Timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	native := dtmcli.NewSaga(c.cfg.Server, saga.GID)
	native.WaitResult = waitResult
	native.BranchHeaders = c.branchHeaders(saga)
	for _, step := range saga.Steps {
		native.Add(step.Action, step.Compensate, step.Payload)
	}

	c.logger.WithContext(ctx).Info("dtm saga submitting",
		logx.String("gid", saga.GID),
		logx.String("saga", saga.Name),
		logx.Int("step_count", len(saga.Steps)),
		logx.Bool("wait_result", waitResult),
	)

	errCh := make(chan error, 1)
	go func() { errCh <- native.Submit() }()

	var err error
	select {
	case <-ctx.Done():
		err = wrapContextError(ctx.Err(), "dtm saga submit canceled")
	case err = <-errCh:
		if err != nil {
			err = errorx.Unavailable(dtmx.CodeSubmitFailed, "dtm saga submit failed", errorx.WithCause(err), errorx.WithRetryable(true))
		}
	}

	result.Elapsed = time.Since(started)
	result.Submitted = err == nil
	if err != nil {
		c.logger.WithContext(ctx).Error("dtm saga submit failed", logx.String("gid", saga.GID), logx.String("saga", saga.Name), logx.Duration("elapsed", result.Elapsed), logx.Err(err))
		record(ctx, c.cfg, "submit_saga", "error", errorx.CodeOf(err).String(), result.Elapsed)
		return result, err
	}
	c.logger.WithContext(ctx).Info("dtm saga submitted", logx.String("gid", saga.GID), logx.String("saga", saga.Name), logx.Duration("elapsed", result.Elapsed))
	record(ctx, c.cfg, "submit_saga", "ok", errorx.CodeOK.String(), result.Elapsed)
	return result, nil
}
