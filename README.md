# CodeValdComm

Real-time communication microservice for the CodeVald platform. Manages channels, messages,
threads, reactions, read receipts, and participants via gRPC + HTTP on a single port (cmux).

## Architecture

- **gRPC** — `CommService` (GetSchema) + `EntityService` (all CRUD, from SharedLib)
- **HTTP** — 9 convenience handlers for comm-domain flows (send message, edit, react, …)
- **ArangoDB** — entity graph backend via SharedLib's `entitygraph` package
- **CodeValdCross** — optional registration for service discovery and event routing

All entity/relationship CRUD is delegated to `entitygraph.DataManager`; the schema is defined
once in [`schema.go`](schema.go) via `DefaultCommSchema()`.

## Running locally

```bash
cp .env.example .env        # fill in ArangoDB credentials
make dev                    # build + run with local defaults
```

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `CODEVALDCOMM_GRPC_PORT` | `50057` | TCP port (gRPC + HTTP via cmux) |
| `COMM_ARANGO_ENDPOINT` | — | ArangoDB endpoint (e.g. `http://localhost:8529`) |
| `COMM_ARANGO_DATABASE` | — | ArangoDB database name |
| `COMM_ARANGO_USER` | — | ArangoDB username |
| `COMM_ARANGO_PASSWORD` | — | ArangoDB password |
| `CODEVALDCOMM_AGENCY_ID` | — | Agency scope; enables startup schema seed |
| `CROSS_GRPC_ADDR` | — | CodeValdCross gRPC address; leave unset for standalone mode |
| `CROSS_PING_INTERVAL` | `20s` | Heartbeat interval for Cross registration |
| `CROSS_PING_TIMEOUT` | `5s` | Per-RPC timeout for Cross registration calls |
| `COMM_GRPC_ADVERTISE_ADDR` | listen addr | Address Cross uses to dial back into this service |

See [`.env.example`](.env.example) for full documentation.

## Make targets

```
make build          # verify module compiles
make build-server   # build production binary → bin/codevaldcomm
make build-dev      # build dev binary → bin/codevaldcomm-dev
make dev            # build-dev + run with .env sourced
make server         # build-server + run (no .env)
make proto          # regenerate gRPC stubs (requires buf)
make test           # unit tests with race detector
make test-arango    # ArangoDB integration tests
make test-all       # unit + integration tests
make cover          # tests + HTML coverage report
make vet            # go vet
make lint           # golangci-lint
make clean          # remove binaries and coverage files
```

## Proto codegen

```bash
# Install tools (once)
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

make proto
```

## Docker

Build from the **monorepo root** (required for the SharedLib `replace` directive):

```bash
docker build -f CodeValdComm/Dockerfile.server -t codevaldcomm:local ../..
```
