.PHONY: build install test test-v lint clean validate fmt vet run

BINARY := forge
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/forge

install:
	go install ./cmd/forge

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
	rm -rf $(BUILD_DIR) coverage.out

validate:
	gh auth status
	claude --version
	@echo "All prerequisites OK"

# Catch-all so `make run <path>` doesn't error on the path argument.
%:
	@:
