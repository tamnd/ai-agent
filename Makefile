# SPDX-License-Identifier: Apache-2.0

GOLANGCI_LINT_VERSION := v2.15.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build test lint vuln site serve clean

all: test build

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/aik ./cmd/aik

test:
	go vet ./...
	go test -race ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

site:
	go run ./cmd/aik site check -root testdata/corpus -out build/site

serve:
	go run ./cmd/aik site serve -root testdata/corpus -out build/site

clean:
	rm -rf bin build
