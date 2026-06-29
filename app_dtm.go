package kernel

import (
	"context"

	"github.com/aisphereio/kernel/dtmx"
)

func configureDefaultDTM(o *options) dtmx.Manager {
	if o == nil || o.dtm == nil {
		return dtmx.Disabled()
	}
	return o.dtm
}

func (a *App) dtm() dtmx.Manager {
	if a == nil || a.opts.dtm == nil {
		return dtmx.Disabled()
	}
	return a.opts.dtm
}

// DTMFromContext returns the distributed transaction manager injected by
// kernel.App, or a disabled manager when DTM is not configured.
func DTMFromContext(ctx context.Context) dtmx.Manager {
	return dtmx.FromContext(ctx)
}
