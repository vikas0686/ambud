MODULE     := github.com/vikas0686/ambud
BIN_DIR    := bin
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X '$(MODULE)/internal/version.Version=$(VERSION)' \
	-X '$(MODULE)/internal/version.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/version.BuildDate=$(BUILD_DATE)'

GOLANGCI_LINT_VERSION := v2.13.1

.PHONY: all
all: fmt-check vet lint test build ## Run the Go check + build pipeline

.PHONY: ci
ci: fmt-check vet lint test-race build web-install web-format-check web-lint web-build ## Run everything CI runs, locally

.PHONY: build
build: ## Build ambudctl and ambud-agent into bin/
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/ambudctl ./cmd/ambudctl
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/ambud-agent ./cmd/ambud-agent

.PHONY: test
test: ## Run unit tests with coverage
	go test ./... -cover

.PHONY: test-race
test-race: ## Run unit tests with the race detector and coverage
	go test ./... -race -cover

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (install with `make install-tools` if missing)
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format Go source with gofmt -s and goimports
	gofmt -w -s .
	golangci-lint fmt ./...

.PHONY: fmt-check
fmt-check: ## Fail if any Go file is not gofmt -s formatted
	@unformatted="$$(gofmt -l -s .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt -s formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: check-tidy
check-tidy: ## Fail if go.mod/go.sum would change under go mod tidy
	go mod tidy
	@if [ -n "$$(git status --porcelain -- go.mod go.sum)" ]; then \
		echo "go.mod/go.sum are not tidy — run 'make tidy' and commit the result:"; \
		git status --porcelain -- go.mod go.sum; \
		exit 1; \
	fi

.PHONY: install-tools
install-tools: ## Install pinned developer tooling (golangci-lint) into GOBIN
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: web-install
web-install: ## Install web/ dependencies
	cd web && npm ci

.PHONY: web-build
web-build: ## Build the web dashboard
	cd web && npm run build

.PHONY: web-lint
web-lint: ## Lint the web dashboard
	cd web && npm run lint

.PHONY: web-format-check
web-format-check: ## Fail if any web/ file is not Prettier-formatted
	cd web && npm run format:check

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) web/dist

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
