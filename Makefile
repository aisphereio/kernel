.PHONY: help deps tools api proto proto-check generate build run test test-root test-errorx test-logx test-cmd test-race test-integration verify verify-full check check-errorx release-check vet vuln lint lint-ci contract cover cover-html verify-errorx bench-errorx fuzz-errorx clean tidy golangci-lint-install pre-commit-install pre-commit-run

GO ?= go
LOCAL_BIN := $(CURDIR)/.bin
COVERPROFILE ?= coverage.out
FUZZTIME ?= 30s
RELEASE_VERSION ?=
KERNEL_TEST_PACKAGES ?= ./authn ./authz ./accessx ./serverx ./gatewayx ./requestx ./middleware/... ./cmd/...

ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
export PATH := $(LOCAL_BIN);$(PATH)
else
export PATH := $(LOCAL_BIN):$(PATH)
endif

help:
	@echo "Aisphere Kernel repository targets:"
	@echo "  make tools             build Kernel CLI and generator tools into .bin"
	@echo "  make api               generate Kernel protobuf/grpc/http/gateway/openapi code"
	@echo "  make proto-check       run buf lint and buf-check-aisphere on Kernel proto contracts"
	@echo "  make test              run Kernel runtime/tooling tests"
	@echo "  make test-cmd          run command package tests"
	@echo "  make verify            run the normal Kernel repository gate"
	@echo "  make verify-full       run verify plus race/vuln/integration checks"
	@echo "  make run               run kernel CLI help"
	@echo "  make clean             remove generated local artifacts"
	@echo ""
	@echo "Generated service layout targets such as make deploy belong to github.com/aisphereio/kernel-layout."

check: check-errorx vet test
	@echo "✓ all checks passed"

check-errorx:
ifeq ($(OS),Windows_NT)
	@cmd /c scripts\verify-errorx.cmd
else
	@chmod +x scripts/check-errorx-usage.sh 2>/dev/null || true
	@./scripts/check-errorx-usage.sh
endif

release-check:
ifeq ($(OS),Windows_NT)
	@cmd /c scripts\check-release-modules.cmd $(RELEASE_VERSION)
else
	@sh scripts/check-release-modules.sh $(RELEASE_VERSION)
endif

golangci-lint-install:
ifeq ($(OS),Windows_NT)
	@echo "install golangci-lint from https://golangci-lint.run/usage/install/"
else
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
			| sh -s -- -b $$($(GO) env GOPATH)/bin v1.62.0; \
		golangci-lint --version; \
	else \
		echo "golangci-lint already installed: $$(golangci-lint --version)"; \
	fi
endif

lint-ci: golangci-lint-install
	golangci-lint run --timeout 5m

pre-commit-install:
	@pip install pre-commit 2>/dev/null || pip3 install pre-commit
	@pre-commit install
	@echo "pre-commit hooks installed"

pre-commit-run:
	@pre-commit run --all-files

deps:
	$(GO) mod download

tools:
ifeq ($(OS),Windows_NT)
	@cmd /c scripts\tools.cmd
else
	@mkdir -p $(LOCAL_BIN)
	@echo "building local Kernel tools into $(LOCAL_BIN)"
	@if [ -d cmd/kernel ]; then cd cmd/kernel && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/protoc-gen-go-http ]; then cd cmd/protoc-gen-go-http && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/protoc-gen-go-errors ]; then cd cmd/protoc-gen-go-errors && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/protoc-gen-go-authz ]; then cd cmd/protoc-gen-go-authz && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/protoc-gen-go-gateway ]; then cd cmd/protoc-gen-go-gateway && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/protoc-gen-go-deploy ]; then cd cmd/protoc-gen-go-deploy && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/protoc-gen-go-kernel ]; then cd cmd/protoc-gen-go-kernel && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@if [ -d cmd/buf-check-aisphere ]; then cd cmd/buf-check-aisphere && GOBIN=$(LOCAL_BIN) $(GO) install .; fi
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0
	@if ! command -v buf >/dev/null 2>&1; then GOBIN=$(LOCAL_BIN) $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0; fi
endif

generate: tools
	$(GO) generate ./...

proto: tools
ifeq ($(OS),Windows_NT)
	@if exist .bin\buf.exe (.bin\buf.exe generate) else (buf generate)
else
	@if [ -x "$(LOCAL_BIN)/buf" ]; then $(LOCAL_BIN)/buf generate; else buf generate; fi
endif

api: proto
	@echo "✓ kernel api generation complete"

proto-check: tools
ifeq ($(OS),Windows_NT)
	@if exist .bin\buf.exe (.bin\buf.exe lint) else (buf lint)
	@if exist .bin\buf.exe (.bin\buf.exe build -o - | .bin\buf-check-aisphere.exe) else (buf build -o - | buf-check-aisphere)
else
	@if [ -x "$(LOCAL_BIN)/buf" ]; then $(LOCAL_BIN)/buf lint; else buf lint; fi
	@if [ -x "$(LOCAL_BIN)/buf" ]; then $(LOCAL_BIN)/buf build -o - | $(LOCAL_BIN)/buf-check-aisphere; else buf build -o - | buf-check-aisphere; fi
endif

contract: proto-check

test: test-root

test-root:
	$(GO) test $(KERNEL_TEST_PACKAGES)

test-errorx:
	$(GO) test ./errorx -v

test-logx:
	$(GO) test ./logx -v

test-cmd:
ifeq ($(OS),Windows_NT)
	@cmd /c scripts\test-cmd.cmd
else
	@if [ -d cmd/kernel ]; then cd cmd/kernel && $(GO) test ./...; fi
	@if [ -d cmd/protoc-gen-go-http ]; then cd cmd/protoc-gen-go-http && $(GO) test ./...; fi
	@if [ -d cmd/protoc-gen-go-errors ]; then cd cmd/protoc-gen-go-errors && $(GO) test ./...; fi
	@if [ -d cmd/protoc-gen-go-authz ]; then cd cmd/protoc-gen-go-authz && $(GO) test ./...; fi
	@if [ -d cmd/protoc-gen-go-gateway ]; then cd cmd/protoc-gen-go-gateway && $(GO) test ./...; fi
	@if [ -d cmd/protoc-gen-go-deploy ]; then cd cmd/protoc-gen-go-deploy && $(GO) test ./...; fi
	@if [ -d cmd/protoc-gen-go-kernel ]; then cd cmd/protoc-gen-go-kernel && $(GO) test ./...; fi
	@if [ -d cmd/buf-check-aisphere ]; then cd cmd/buf-check-aisphere && $(GO) test ./...; fi
endif

test-race:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if ((& '$(GO)' env CGO_ENABLED) -eq '1') { & '$(GO)' test -race $(KERNEL_TEST_PACKAGES); exit $$LASTEXITCODE } else { Write-Host 'CGO_ENABLED=0; skipping race detector because Go requires cgo for -race' }"
else
	@if [ "$$($(GO) env CGO_ENABLED)" = "1" ]; then \
		$(GO) test -race $(KERNEL_TEST_PACKAGES); \
	else \
		echo 'CGO_ENABLED=0; skipping race detector because Go requires cgo for -race'; \
	fi
endif

test-integration:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "$$env:KERNEL_INTEGRATION='1'; & '$(GO)' test -tags=integration ./cachex ./dbx -v; exit $$LASTEXITCODE"
else
	KERNEL_INTEGRATION=1 $(GO) test -tags=integration ./cachex ./dbx -v
endif

vet:
	$(GO) vet ./...

vuln:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Get-Command govulncheck -ErrorAction SilentlyContinue) { govulncheck ./...; exit $$LASTEXITCODE } else { Write-Host 'govulncheck not found; install with: go install golang.org/x/vuln/cmd/govulncheck@latest' }"
else
	@if command -v govulncheck >/dev/null 2>&1; then govulncheck ./...; else echo 'govulncheck not found; install with: go install golang.org/x/vuln/cmd/govulncheck@latest'; fi
endif

lint:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Get-Command golangci-lint -ErrorAction SilentlyContinue) { golangci-lint run --timeout 5m; exit $$LASTEXITCODE } else { Write-Host 'golangci-lint not found; run make lint-ci after installing it' }"
else
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run --timeout 5m; else echo 'golangci-lint not found; run make lint-ci after installing it'; fi
endif

cover:
	$(GO) test $(KERNEL_TEST_PACKAGES) -coverprofile $(COVERPROFILE)
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path '$(COVERPROFILE)') { & '$(GO)' tool cover -func='$(COVERPROFILE)'; exit $$LASTEXITCODE } else { Write-Host 'No coverage profile produced' }"
else
	@if [ -f '$(COVERPROFILE)' ]; then $(GO) tool cover -func=$(COVERPROFILE); else echo 'No coverage profile produced'; fi
endif

cover-html: cover
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path '$(COVERPROFILE)') { & '$(GO)' tool cover -html='$(COVERPROFILE)' -o coverage.html; exit $$LASTEXITCODE } else { Write-Host 'No coverage profile produced' }"
else
	@if [ -f '$(COVERPROFILE)' ]; then $(GO) tool cover -html=$(COVERPROFILE) -o coverage.html; else echo 'No coverage profile produced'; fi
endif

run:
ifeq ($(OS),Windows_NT)
	@if exist .bin\kernel.exe (.bin\kernel.exe --help) else (cd cmd\kernel && $(GO) run . --help)
else
	@if [ -x "$(LOCAL_BIN)/kernel" ]; then \
		"$(LOCAL_BIN)/kernel" --help; \
	elif [ -d cmd/kernel ]; then \
		cd cmd/kernel && $(GO) run . --help; \
	else \
		echo 'cmd/kernel not found'; \
	fi
endif

tidy:
	$(GO) mod tidy

build: tools
	@echo "kernel tools built into $(LOCAL_BIN)"

verify: deps proto-check test test-cmd vet cover

verify-full: verify test-race vuln test-integration

verify-errorx:
ifeq ($(OS),Windows_NT)
	@cmd /c scripts\verify-errorx.cmd
else
	bash ./scripts/verify-errorx.sh
endif

bench-errorx:
	$(GO) test ./errorx -bench=.

fuzz-errorx:
	$(GO) test ./errorx -run=^$$ -fuzz=FuzzNewError -fuzztime=$(FUZZTIME)

clean:
ifeq ($(OS),Windows_NT)
	@if exist .bin rmdir /s /q .bin
	@if exist $(COVERPROFILE) del /f /q $(COVERPROFILE)
	@if exist coverage.html del /f /q coverage.html
else
	rm -rf $(LOCAL_BIN)
	rm -f $(COVERPROFILE) coverage.html
endif
