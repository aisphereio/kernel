// Package dtmx wraps DTM HTTP Saga orchestration for kernel services.
//
// DTM itself remains an external coordinator. dtmx provides:
//   - kernel config / logx / errorx / metricsx integration
//   - saga GID generation
//   - HTTP /api/dtmsvr/submit client
//   - branch URL construction for application-local branch handlers
//
// Services should expose idempotent action and compensate endpoints, and keep
// large payloads out of DTM's payload table. Store large data in object storage
// first, then pass only object keys and hashes in the saga payload.
package dtmx
