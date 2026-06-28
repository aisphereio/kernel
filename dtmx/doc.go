// Package dtmx defines Kernel's distributed transaction abstraction.
//
// dtmx is intentionally provider-neutral. Business packages depend on this
// package only, while concrete implementations live in subpackages such as
// dtmx/dtm. The first production implementation is backed by dtm-labs/dtm via
// the official Go HTTP SDK.
//
// Import an implementation once during bootstrap:
//
//	import _ "github.com/aisphereio/kernel/dtmx/dtm"
//
// Then open it from config:
//
//	manager, err := dtmx.New(dtmx.Config{
//	    Enabled: true,
//	    Driver:  "dtm",
//	    Server:  "http://127.0.0.1:36789/api/dtmsvr",
//	})
//
// Kernel services should model cross-resource workflows as Saga branches with
// idempotent action and compensation endpoints. Large binary payloads must not
// be put into DTM request bodies; put objects into object storage first and pass
// only object keys, hashes, sizes, and metadata through Saga payloads.
package dtmx
