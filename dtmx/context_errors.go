package dtmx

import "context"

func init() {
	contextCanceledSentinel = context.Canceled
	contextTimeoutSentinel = context.DeadlineExceeded
}
