#!/usr/bin/env bash
set -euo pipefail

if grep -R "github.com/aisphereio/kernel/errors" --include='*.go' .; then
  echo "legacy kernel/errors import found" >&2
  exit 1
fi

if [ -d ./errors ]; then
  echo "legacy root errors package still exists" >&2
  exit 1
fi

if grep -R "package errors" ./errors --include='*.go' 2>/dev/null; then
  echo "legacy root errors package found" >&2
  exit 1
fi

if ! [ -d ./cmd/protoc-gen-go-errors ]; then
  echo "proto error generator is missing; it should be retained and generate errorx helpers" >&2
  exit 1
fi

if ! [ -f ./third_party/errors/errors.proto ]; then
  echo "third_party/errors/errors.proto is missing; proto enum option compatibility is required" >&2
  exit 1
fi

if grep -R "kernel/errors" cmd/protoc-gen-go-errors third_party/errors --include='*.go' --include='*.proto' --include='*.tpl' 2>/dev/null; then
  echo "proto errorx generator still references legacy kernel/errors" >&2
  exit 1
fi

if ! grep -R "github.com/aisphereio/kernel/errorx" cmd/protoc-gen-go-errors --include='*.go' --include='*.tpl' >/dev/null 2>&1; then
  echo "protoc-gen-go-errors must generate errorx helpers" >&2
  exit 1
fi

go test ./errorx -v
go test ./errorx -race
go test ./errorx -cover
if [ -d cmd/protoc-gen-go-errors ]; then (cd cmd/protoc-gen-go-errors && go test ./...); fi
go test ./...
go vet ./...
go run ./examples/errorx-basic
go test ./errorx -bench=.
