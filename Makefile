.PHONY: help deps tools test test-root test-errorx test-logx test-cmd test-race cover cover-html vet vuln lint contract verify verify-errorx bench-errorx fuzz-errorx clean proto generate

GO ?= go
LOCAL_BIN := $(CURDIR)/.bin
COVERPROFILE ?= coverage.out
FUZZTIME ?= 30s
ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
export PATH := $(LOCAL_BIN);$(PATH)
else
export PATH := $(LOCAL_BIN):$(PATH)
endif

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
	@echo "  make proto         run buf generate using local .bin"
	@echo "  make clean         remove generated local artifacts"

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
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
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

cover-html: cover
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path '$(COVERPROFILE)') { & '$(GO)' tool cover -html='$(COVERPROFILE)' -o coverage.html; exit $$LASTEXITCODE } else { Write-Host 'No coverage profile produced' }"
else
	@if [ -f '$(COVERPROFILE)' ]; then $(GO) tool cover -html=$(COVERPROFILE) -o coverage.html; else echo 'No coverage profile produced'; fi
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

verify: test test-cmd test-race vet cover vuln lint contract

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
