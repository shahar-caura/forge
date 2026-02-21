.PHONY: build install test test-v lint clean validate fmt vet run release-dry generate web web-lint web-test lint-spec test-api

BINARY := forge
BUILD_DIR := bin

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build: web
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/forge

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/forge

run: build
	./$(BUILD_DIR)/$(BINARY) run $(filter-out $@,$(MAKECMDGOALS))

test:
	go test -race ./...

test-v:
	go test -race -v ./...

lint:
	golangci-lint run ./...

fmt:
	goimports -w .

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out dist web/dist web/node_modules

validate:
	gh auth status
	claude --version
	@echo "All prerequisites OK"

release-dry:
	goreleaser release --snapshot --clean

generate:
	oapi-codegen -config api/oapi-codegen.yaml -exclude-operation-ids streamEvents api/openapi.yaml

web:
	cd web && npm ci && npm run build

web-lint:
	cd web && npm run lint && npm run format:check

web-test:
	cd web && npm run test

lint-spec:
	npx @stoplight/spectral-cli lint api/openapi.yaml --ruleset api/.spectral.yaml

test-api:
	@echo "Starting server for contract tests..."
	@PORT=$$(python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()'); \
	./$(BUILD_DIR)/$(BINARY) serve --port $$PORT & SERVER_PID=$$!; \
	sleep 1; \
	schemathesis run api/openapi.yaml --base-url http://localhost:$$PORT/api --mode=all; \
	EXIT=$$?; kill $$SERVER_PID 2>/dev/null; exit $$EXIT

# Catch-all so `make run <path>` doesn't error on the path argument.
%:
	@:
