SHELL := /usr/bin/env bash

BINARY      := khealth
PKG         := github.com/neilfarmer/k8s-health
CMD_DIR     := ./cmd/khealth
DIST_DIR    := dist
COVER_FILE  := coverage.out

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Commit=$(COMMIT) \
  -X $(PKG)/internal/version.Date=$(DATE)

GO ?= go
GOFLAGS ?=

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Run go mod tidy
	$(GO) mod tidy

.PHONY: fmt
fmt: ## Format Go sources
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: test
test: ## Run unit tests with race detector
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## Run unit tests and write coverage profile
	$(GO) test -race -count=1 -covermode=atomic -coverprofile=$(COVER_FILE) ./...
	$(GO) tool cover -func=$(COVER_FILE) | tail -1

.PHONY: build
build: ## Build the binary into ./dist
	mkdir -p $(DIST_DIR)
	$(GO) build $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' -o $(DIST_DIR)/$(BINARY) $(CMD_DIR)

.PHONY: install
install: ## Install the binary into $$GOBIN
	$(GO) install $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' $(CMD_DIR)

.PHONY: run
run: ## Run khealth from source
	$(GO) run $(CMD_DIR) $(ARGS)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(DIST_DIR) $(COVER_FILE)

.PHONY: vuln
vuln: ## Check known vulnerabilities in deps with govulncheck
	govulncheck ./...

.PHONY: gosec
gosec: ## Static security analysis with gosec
	gosec -quiet -severity medium -confidence medium ./...

.PHONY: security
security: vuln gosec ## Run all security checks

.PHONY: integration
integration: ## Run integration tests against a local kind cluster (requires kind + kubectl)
	./test/integration/run.sh

.PHONY: docker
docker: ## Build container image (uses goreleaser-style Dockerfile)
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg DATE=$(DATE) \
	  -t ghcr.io/neilfarmer/k8s-health:$(VERSION) .

.PHONY: release-snapshot
release-snapshot: ## Build a snapshot release with goreleaser (no publish)
	goreleaser release --snapshot --clean

.PHONY: ci
ci: tidy vet lint test ## Local equivalent of the CI workflow
