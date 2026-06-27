# Aisphere Kernel Agent Guide

You are working inside Aisphere Kernel.

Aisphere Kernel is an AI-native Go application kernel.
It owns application lifecycle, config, logging, errors, HTTP, gRPC, context, authz, audit, metrics, tracing, DB, Redis, S3, events, workers, and module registration.

## Prime directive

Do not invent infrastructure.
Do not create custom loggers, HTTP servers, DB clients, Redis clients, S3 clients, auth middleware, error formats, or audit logic.
Use Kernel APIs only.

## Allowed work

You may create or edit:
- business modules
- handlers
- services
- repositories
- DTOs
- tests
- migrations
- examples

## Forbidden work

Do not directly import:
- database/sql
- net/http server setup
- go-redis
- minio SDK
- zap/slog/logrus directly
- casdoor SDK
- casbin enforcer
- os.Getenv inside business code

## Standard flow

Handler → Service → Repository

Handler:
- bind input
- call service
- return response

Service:
- validate business rules
- call ctx.Authz().Check()
- call repository
- record audit event for sensitive operations
- return errorx errors only

Repository:
- use kernel dbx
- no HTTP/auth/audit logic

## Before finishing

Run:
- go test ./...
- kernel-lint ./...
- kernel-contract-check ./...

## errorx module rules

When returning business errors, use `github.com/aisphereio/kernel/errorx`.

Use:
- `errorx.BadRequest` for validation errors
- `errorx.Unauthorized` for unauthenticated requests
- `errorx.Forbidden` for permission denied
- `errorx.NotFound` for missing resources
- `errorx.Conflict` for duplicate/conflict state
- `errorx.Unavailable` or `errorx.Timeout` for dependency failures
- `errorx.Wrap` to preserve infrastructure causes

Do not use `errors.New`, `fmt.Errorf`, or `panic` for business errors in handler/service/repository code. Dynamic values must go into metadata, not error code.
