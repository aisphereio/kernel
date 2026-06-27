// Package auditx defines durable audit recording contracts.
//
// auditx is separate from logx. logx emits operational telemetry; auditx writes
// append-only security records for authentication, authorization and privileged
// business mutations. Implementations can write to PostgreSQL, Kafka,
// ClickHouse, object storage or another compliance pipeline.
package auditx
