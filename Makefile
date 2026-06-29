.PHONY: check check-errorx release-check golangci-lint-install pre-commit-install pre-commit-run help deps tools api wire build run test test-integration verify-full test-root test-errorx test-logx test-cmd test-race cover cover-html vet vuln lint contract verify verify-errorx bench-errorx fuzz-errorx clean proto generate

help:
	@echo "Aisphere Kernel targets:"
	@echo "  make tools         build local tools into .bin"
	@echo "  make deps          download root module dependencies"
	@echo "  make test          run root module tests"
	@echo "  make test-cmd      run command submodule tests"
	@echo "  make test-errorx   run errorx tests"
	@echo "  make test-logx     run logx tests"
	@echo "  make test-race     run root tests with race detector"
	@echo "  make cover         generate coverage profile and summary"
	@echo "  make vet           run go vet on root module"
	@echo "  make vuln          run govulncheck when installed"
	@echo "  make lint          run kernel-lint when installed/built"
	@echo "  make verify        run local verification gate"
	@echo "  make verify-errorx run errorx acceptance checks"
	@echo "  make release-check verify command module release layout"
	@echo "  make proto         run buf generate using local .bin"
	@echo "  make clean         remove generated local artifacts"
	@echo "  make api           alias of proto, generate api code"
	@echo "  make wire          generate wire dependency injection code when wire.go exists"
	@echo "  make build         build kernel local tools"
	@echo "  make run           run kernel cli help"
	@echo "  make tidy          run go mod tidy"
	@echo "  make test-integration  run Docker/Testcontainers integration tests"
	@echo "  make verify-full       run verify plus integration tests"

# ---- One-command check: errorx usage + lint + vet + test ----
check: check-errorx vet test
	@echo "✓ all checks passed"

# ---- errorx usage grep check (fast, no Go toolchain needed) ----
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

# ---- Install golangci-lint (one-time) ----
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

# ---- Run golangci-lint (depguard enforces errorx/logx usage) ----
lint-ci: golangci-lint-install
	golangci-lint run --timeout 5m

# ---- Install pre-commit hooks (one-time) ----
pre-commit-install:
	@pip install pre-commit 2>/dev/null || pip3 install pre-commit
	@pre-commit install
	@echo "pre-commit hooks installed"

# ---- Run pre-commit on all files (manual) ----
pre-commit-run:
	@pre-commit run --all-files

# ---- Update help text ----
# Add these lines to the help target:
#	@echo "  make check          run all checks (errorx + vet + test)"
#	@echo "  make check-errorx   run errorx usage grep check"
#	@echo "  make lint-ci        run golangci-lint (depguard)"
#	@echo "  make pre-commit-install  install pre-commit hooks"
#	@echo "  make pre-commit-run      run pre-commit on all files"



GO ?= go
LOCAL_BIN := $(CURDIR)/.bin
COVERPROFILE ?= coverage.out
FUZZTIME ?= 30s
RELEASE_VERSION ?=
ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
export PATH := $(LOCAL_BIN);$(PATH)
else
export PATH := $(LOCAL_BIN):$(PATH)
endif


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
	buf generate
endif

api: proto
	@echo "✓ api generation complete: protobuf, grpc, kernel http, grpc-gateway, openapi"

test: test-root

test-root:
	$(GO) test ./...

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
endif

test-race:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if ((& '$(GO)' env CGO_ENABLED) -eq '1') { & '$(GO)' test -race ./...; exit $$LASTEXITCODE } else { Write-Host 'CGO_ENABLED=0; skipping race detector because Go requires cgo for -race' }"
else
	@if [ "$$($(GO) env CGO_ENABLED)" = "1" ]; then \
		$(GO) test -race ./...; \
	else \
		echo 'CGO_ENABLED=0; skipping race detector because Go requires cgo for -race'; \
	fi
endif

cover:
	$(GO) test ./... -coverprofile $(COVERPROFILE)
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path '$(COVERPROFILE)') { & '$(GO)' tool cover -func='$(COVERPROFILE)'; exit $$LASTEXITCODE } else { Write-Host 'No coverage profile produced' }"
else
	@if [ -f '$(COVERPROFILE)' ]; then $(GO) tool cover -func=$(COVERPROFILE); else echo 'No coverage profile produced'; fi
endif

verify-full: verify test-integration

cover-html: cover
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path '$(COVERPROFILE)') { & '$(GO)' tool cover -html='$(COVERPROFILE)' -o coverage.html; exit $$LASTEXITCODE } else { Write-Host 'No coverage profile produced' }"
else
	@if [ -f '$(COVERPROFILE)' ]; then $(GO) tool cover -html=$(COVERPROFILE) -o coverage.html; else echo 'No coverage profile produced'; fi
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
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "$$local = Join-Path '.bin' 'kernel-lint.exe'; if (Test-Path $$local) { & $$local ./...; exit $$LASTEXITCODE } elseif (Get-Command kernel-lint -ErrorAction SilentlyContinue) { kernel-lint ./...; exit $$LASTEXITCODE } else { Write-Host 'kernel-lint not found; skipping until tools/kernel-lint is implemented' }"
else
	@if [ -x "$(LOCAL_BIN)/kernel-lint" ]; then $(LOCAL_BIN)/kernel-lint ./...; \
	elif command -v kernel-lint >/dev/null 2>&1; then kernel-lint ./...; \
	else echo 'kernel-lint not found; skipping until tools/kernel-lint is implemented'; fi
endif

contract:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Get-Command kernel-contract-check -ErrorAction SilentlyContinue) { kernel-contract-check ./...; exit $$LASTEXITCODE } else { Write-Host 'kernel-contract-check not found; skipping until tools/kernel-contract-check is implemented' }"
else
	@if command -v kernel-contract-check >/dev/null 2>&1; then kernel-contract-check ./...; else echo 'kernel-contract-check not found; skipping until tools/kernel-contract-check is implemented'; fi
endif

verify: api wire test test-cmd test-race vet cover vuln lint contract

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

wire:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "$$done = $$false; if (Test-Path 'cmd/kernel/wire.go') { Push-Location 'cmd/kernel'; & '$(GO)' run github.com/google/wire/cmd/wire@v0.7.0 .; $$code = $$LASTEXITCODE; Pop-Location; if ($$code -ne 0) { exit $$code }; $$done = $$true }; if (Test-Path 'internal/app/wire.go') { & '$(GO)' run github.com/google/wire/cmd/wire@v0.7.0 ./internal/app; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; $$done = $$true }; if (-not $$done) { Write-Host 'no wire.go found; skipping wire' }"
else
	@if [ -f cmd/kernel/wire.go ]; then \
		cd cmd/kernel && $(GO) run github.com/google/wire/cmd/wire@v0.7.0 .; \
	elif [ -f internal/app/wire.go ]; then \
		$(GO) run github.com/google/wire/cmd/wire@v0.7.0 ./internal/app; \
	else \
		echo 'no wire.go found; skipping wire'; \
	fi
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
