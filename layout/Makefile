GO ?= go
BUF ?= buf

APP_NAME ?= server
APP_CMD ?= ./cmd/$(APP_NAME)
CONF ?= ./configs/config.yaml
RUN_ARGS ?= -conf $(CONF)

LOCAL_BIN := $(CURDIR)/.bin
BIN_DIR := $(CURDIR)/bin
COVERPROFILE ?= coverage.out

VERSION ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)

ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
BIN_DIR := $(CURDIR)\bin
export PATH := $(LOCAL_BIN);$(PATH)
else
export PATH := $(LOCAL_BIN):$(PATH)
endif

.PHONY: help tools api config wire generate build run test tidy verify clean

help:
	@echo "Kernel service targets:"
	@echo "  make tools       install local project tools into .bin"
	@echo "  make api         generate api proto code by buf.gen.yaml"
	@echo "  make config      generate internal config proto code if buf.gen.config.yaml exists"
	@echo "  make wire        generate dependency injection code"
	@echo "  make generate    run go generate"
	@echo "  make build       build service binary"
	@echo "  make run         run service locally"
	@echo "  make test        run all tests"
	@echo "  make tidy        run go mod tidy"
	@echo "  make verify      run api, config, wire, generate, tidy, test, build"
	@echo "  make clean       clean local artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  APP_NAME=$(APP_NAME)"
	@echo "  APP_CMD=$(APP_CMD)"
	@echo "  CONF=$(CONF)"
	@echo ""
	@echo "Examples:"
	@echo "  make tools KERNEL_ROOT=../kernel"
	@echo "  make run APP_NAME=aispherehub APP_CMD=./cmd/aispherehub"

tools:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
ifdef KERNEL_ROOT
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& cd /d $(KERNEL_ROOT)\cmd\kernel && $(GO) install ."
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& cd /d $(KERNEL_ROOT)\cmd\protoc-gen-go-http && $(GO) install ."
	@cmd /c "if exist $(KERNEL_ROOT)\cmd\protoc-gen-go-errors set GOBIN=$(LOCAL_BIN)&& cd /d $(KERNEL_ROOT)\cmd\protoc-gen-go-errors && $(GO) install ."
else
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/aisphereio/kernel/cmd/kernel@latest"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/aisphereio/kernel/cmd/protoc-gen-go-http@latest"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/aisphereio/kernel/cmd/protoc-gen-go-errors@latest"
endif
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/google/wire/cmd/wire@v0.7.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/google/gnostic/cmd/protoc-gen-openapi@v0.7.1"
else
	@mkdir -p $(LOCAL_BIN)
ifdef KERNEL_ROOT
	@cd $(KERNEL_ROOT)/cmd/kernel && GOBIN=$(LOCAL_BIN) $(GO) install .
	@cd $(KERNEL_ROOT)/cmd/protoc-gen-go-http && GOBIN=$(LOCAL_BIN) $(GO) install .
	@if [ -d "$(KERNEL_ROOT)/cmd/protoc-gen-go-errors" ]; then cd $(KERNEL_ROOT)/cmd/protoc-gen-go-errors && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
else
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/aisphereio/kernel/cmd/kernel@latest
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/aisphereio/kernel/cmd/protoc-gen-go-http@latest
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/aisphereio/kernel/cmd/protoc-gen-go-errors@latest
endif
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/google/wire/cmd/wire@v0.7.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/google/gnostic/cmd/protoc-gen-openapi@v0.7.1
endif

api: tools
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist .bin\buf.exe (.bin\buf.exe generate --template buf.gen.yaml) else ($(BUF) generate --template buf.gen.yaml)"
else
	@$(BUF) generate --template buf.gen.yaml
endif

config: tools
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist buf.gen.config.yaml (if exist .bin\buf.exe (.bin\buf.exe generate --template buf.gen.config.yaml) else ($(BUF) generate --template buf.gen.config.yaml)) else (echo buf.gen.config.yaml not found; skip config)"
else
	@if [ -f buf.gen.config.yaml ]; then $(BUF) generate --template buf.gen.config.yaml; else echo "buf.gen.config.yaml not found; skip config"; fi
endif

wire: tools
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist .bin\wire.exe (.bin\wire.exe ./cmd/$(APP_NAME)) else ($(GO) run github.com/google/wire/cmd/wire@v0.7.0 ./cmd/$(APP_NAME))"
else
	@if [ -x "$(LOCAL_BIN)/wire" ]; then $(LOCAL_BIN)/wire ./cmd/$(APP_NAME); else $(GO) run github.com/google/wire/cmd/wire@v0.7.0 ./cmd/$(APP_NAME); fi
endif

generate:
	$(GO) generate ./...

build:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist bin mkdir bin"
	$(GO) build -ldflags "-X main.Version=$(VERSION)" -o bin\$(APP_NAME).exe $(APP_CMD)
else
	@mkdir -p bin
	$(GO) build -ldflags "-X main.Version=$(VERSION)" -o bin/$(APP_NAME) $(APP_CMD)
endif

run:
	$(GO) run $(APP_CMD) $(RUN_ARGS)

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

verify: api config wire generate tidy test build

clean:
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist .bin rmdir /s /q .bin"
	@cmd /c "if exist bin rmdir /s /q bin"
	@cmd /c "if exist $(COVERPROFILE) del /f /q $(COVERPROFILE)"
	@cmd /c "if exist coverage.html del /f /q coverage.html"
else
	rm -rf $(LOCAL_BIN)
	rm -rf bin
	rm -f $(COVERPROFILE) coverage.html
endif