.PHONY: build install test test-v lint clean validate fmt vet run release-dry node-server-run node-server-test

BINARY := forge
BUILD_DIR := bin

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
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
	rm -rf $(BUILD_DIR) coverage.out dist

validate:
	gh auth status
	claude --version
	@echo "All prerequisites OK"

release-dry:
	goreleaser release --snapshot --clean

node-server-run:
	cd hello-world-server && node server.js

node-server-test:
	@echo "Starting node server for testing..."
	cd hello-world-server && node server.js & \
	SERVER_PID=$$!; \
	echo "Server PID: $$SERVER_PID"; \
	cd hello-world-server && node test.js || (kill $$SERVER_PID && exit 1); \
	kill $$SERVER_PID; \
	echo "Test complete and server stopped."

# Catch-all so `make run <path>` doesn't error on the path argument.
%:
	@:
