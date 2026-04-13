.PHONY: build test lint clean acceptance-test coverage

BINARY := k8s-health
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
  -X github.com/neilfarmer/k8s-health/cmd.version=$(VERSION) \
  -X github.com/neilfarmer/k8s-health/cmd.commit=$(COMMIT) \
  -X github.com/neilfarmer/k8s-health/cmd.date=$(DATE)

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -race -count=1 ./...

coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

acceptance-test:
	go test -tags acceptance -v -timeout 300s ./test/acceptance/...

clean:
	rm -f $(BINARY) coverage.out coverage.html
