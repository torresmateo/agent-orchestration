.PHONY: all build build-harness clean test lint

BINARY_DIR := bin
SHARED_DIR := $(HOME)/.agentvm/shared/bin

all: build build-harness

build:
	go build -o $(BINARY_DIR)/agentctl ./cmd/agentctl
	go build -o $(BINARY_DIR)/agentd ./cmd/agentd

build-harness:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BINARY_DIR)/agent-harness ./cmd/agent-harness

install: build build-harness
	mkdir -p $(SHARED_DIR)
	cp $(BINARY_DIR)/agentctl /usr/local/bin/agentctl
	cp $(BINARY_DIR)/agentd /usr/local/bin/agentd
	cp $(BINARY_DIR)/agent-harness $(SHARED_DIR)/agent-harness

install-harness: build-harness
	mkdir -p $(SHARED_DIR)
	cp $(BINARY_DIR)/agent-harness $(SHARED_DIR)/agent-harness

clean:
	rm -rf $(BINARY_DIR)

test:
	go test ./...

lint:
	golangci-lint run ./...
