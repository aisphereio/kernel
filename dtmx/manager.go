package dtmx

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

func New(cfg Config) (Manager, error) {
	cfg = cfg.withDefaults()
	registerMetrics(cfg)
	logger := componentLogger(cfg)

	if !cfg.Enabled {
		logger.Info("dtmx disabled")
		return Disabled(), nil
	}
	if err := cfg.Validate(); err != nil {
		wrapped := normalizeError(err, "dtmx config invalid")
		logger.Error("dtmx config invalid", logx.Err(wrapped), logx.String("driver", cfg.Driver), logx.String("protocol", cfg.Protocol))
		return nil, wrapped
	}

	driversMu.RLock()
	opener := drivers[cfg.Driver]
	driversMu.RUnlock()
	if opener == nil {
		err := normalizeError(ErrUnknownDriver, "dtmx driver not registered", errorx.WithMetadata("driver", cfg.Driver))
		logger.Error("dtmx driver not registered", logx.Err(err), logx.String("driver", cfg.Driver))
		return nil, err
	}

	started := time.Now()
	logger.Info("dtmx opening", logx.String("driver", cfg.Driver), logx.String("protocol", cfg.Protocol), logx.String("server", cfg.Server), logx.Bool("metrics_enabled", cfg.MetricsEnabled))
	manager, err := opener(cfg)
	elapsed := time.Since(started)
	if err != nil {
		wrapped := normalizeError(err, "dtmx open failed", errorx.WithMetadata("driver", cfg.Driver))
		logger.Error("dtmx open failed", logx.Duration("elapsed", elapsed), logx.Err(wrapped))
		recordOperation(context.Background(), cfg, "open", "error", errorx.CodeOf(wrapped).String(), elapsed)
		return nil, wrapped
	}
	if manager == nil {
		manager = Disabled()
	}
	logger.Info("dtmx opened", logx.String("driver", manager.DriverName()), logx.Duration("elapsed", elapsed))
	recordOperation(context.Background(), cfg, "open", "ok", errorx.CodeOK.String(), elapsed)
	return manager, nil
}

func BuildBranchURL(base, prefix, branchPath string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	prefix = strings.Trim(prefix, "/")
	branchPath = strings.Trim(branchPath, "/")
	if base == "" {
		return ""
	}
	if prefix == "" {
		return base + "/" + branchPath
	}
	return base + "/" + path.Join(prefix, branchPath)
}
