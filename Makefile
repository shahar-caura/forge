.PHONY: build test lint clean validate

BINARY := forge
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/forge

test:
	go test ./...

test-v:
	go test -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

validate:
	gh auth status
	claude --version
	@echo "All prerequisites OK"
