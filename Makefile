.PHONY: build build-server build-dev server dev dev-restart kill proto test cover test-arango test-all vet lint clean

export PATH := /usr/local/go/bin:$(PATH)

# ── Build ─────────────────────────────────────────────────────────────────────

## Verify the module compiles cleanly.
build:
	go build ./...

## Build the production server binary to bin/codevaldcomm-server.
build-server:
	go build -o bin/codevaldcomm-server ./cmd/server

## Build the dev binary to bin/codevaldcomm-dev.
build-dev:
	go build -o bin/codevaldcomm-dev ./cmd/dev

## Run the production server locally. Expects env vars to be set by the caller
## (or the shell) — does not source .env, to mirror container behaviour.
server: build-server
	./bin/codevaldcomm-server

## Run the dev binary with local-dev defaults sourced from .env (if present).
dev: build-dev
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	./bin/codevaldcomm-dev

## Stop any running dev instance, rebuild, and run.
dev-restart: kill dev

## Stop any running instances of the codevaldcomm binaries.
kill:
	@echo "Stopping any running instances..."
	-@pkill -9 -f "bin/codevaldcomm-" 2>/dev/null || true
	@sleep 1

## Stop any running instance, rebuild, and run.
restart: dev-restart

# ── Proto Codegen ─────────────────────────────────────────────────────────────

## Regenerate Go stubs from proto/codevaldcomm/v1/*.proto.
## Requires: buf, protoc-gen-go, protoc-gen-go-grpc on PATH.
## Install: go install github.com/bufbuild/buf/cmd/buf@latest
##          go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
##          go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto:
	buf generate

# ── Tests ─────────────────────────────────────────────────────────────────────

## Run all unit tests with race detector (skips integration tests that need ArangoDB).
test:
	go test -v -race -count=1 ./...

## Run tests and produce an HTML coverage report (coverage.html).
cover:
	go test -v -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## Run ArangoDB integration tests.
## Loads .env if it exists, otherwise falls back to environment variables.
## Usage: make test-arango
##        COMM_ARANGO_ENDPOINT=http://host:8529 COMM_ARANGO_USER=root COMM_ARANGO_PASSWORD=pw make test-arango
test-arango:
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	go test -v -race -count=1 -tags=integration ./storage/arangodb/

## Run everything: unit tests + ArangoDB integration tests.
test-all:
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	go test -v -race -count=1 -tags=integration ./...

# ── Quality ───────────────────────────────────────────────────────────────────

vet:
	go vet ./...

lint:
	golangci-lint run ./...

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	go clean ./...
	rm -rf bin/
	rm -f coverage.out coverage.html
