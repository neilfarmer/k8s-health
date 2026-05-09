SHELL := /usr/bin/env bash

BINARY      := khealth
PKG         := github.com/neilfarmer/k8s-health
CMD_DIR     := ./cmd/khealth
DIST_DIR    := dist
COVER_FILE  := coverage.out
COVER_MIN   ?= 80

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Commit=$(COMMIT) \
  -X $(PKG)/internal/version.Date=$(DATE)

GO ?= go
GOFLAGS ?=

# Pin the toolchain explicitly. When the local go binary is older than the
# toolchain directive in go.mod, the auto bootstrap can flake on -coverpkg
# cross-package coverage with "no such tool covdata" for packages that have
# no test files. Pinning avoids the re-exec and the quirk.
export GOTOOLCHAIN ?= go1.26.3

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

# Packages contributing to the unit-test coverage gate. internal/etcd is
# deliberately excluded — its direct-gRPC and in-cluster-Job code paths are
# only meaningfully exercised against a real cluster (covered in the
# `integration` workflow). See ADR-0008.
COVER_PKGS = $(shell $(GO) list ./... | grep -v '/internal/etcd' | paste -sd, -)

.PHONY: cover
cover: ## Run unit tests and write coverage profile (cross-package)
	$(GO) test -race -count=1 -covermode=atomic -coverpkg=$(COVER_PKGS) -coverprofile=$(COVER_FILE) ./...
	$(GO) tool cover -func=$(COVER_FILE) | tail -1

.PHONY: cover-html
cover-html: cover ## Open the coverage report in a browser
	$(GO) tool cover -html=$(COVER_FILE)

.PHONY: cover-check
cover-check: cover ## Fail if total coverage is below COVER_MIN (default 80)
	@$(GO) tool cover -func=$(COVER_FILE) | \
	  awk -v min=$(COVER_MIN) '/^total:/ { \
	    pct=$$3; gsub("%","",pct); \
	    if (pct+0 < min) { \
	      printf "FAIL: total coverage %s%% is below threshold %s%%\n", pct, min; \
	      exit 1; \
	    } \
	    printf "OK: total coverage %s%% meets threshold %s%%\n", pct, min; \
	  }'

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

.PHONY: local-up
local-up: ## Stand up the kind cluster used by integration tests; build + load image
	./hack/local-kind.sh up

.PHONY: local-down
local-down: ## Delete the local kind cluster
	./hack/local-kind.sh down

.PHONY: local-demo
local-demo: ## local-up + apply broken resources + run `khealth check cluster`
	./hack/local-kind.sh demo

.PHONY: local-test
local-test: ## Run `make integration` against the running local cluster (keeps it)
	./hack/local-kind.sh test

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
ci: tidy vet lint cover-check ## Local equivalent of the CI workflow
