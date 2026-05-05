---
applyTo: '**'
---

# CodeValdComm — Code Structure Rules

## Service Design Principles

CodeValdComm is a **gRPC + HTTP microservice** for real-time communication (channels, messages,
threads, reactions, read receipts, participants). These rules reflect that:

- **All entity/relationship CRUD is delegated to `entitygraph.DataManager`** — never write
  ArangoDB queries directly in service or handler code
- **All schema management is delegated to `entitygraph.SchemaManager`** — the schema is defined
  once in `schema.go` via `DefaultCommSchema()`
- **CommService proto exposes only `GetSchema`** — all entity operations are served by
  `EntityService` from SharedLib
- **HTTP convenience handlers publish domain events** via `CrossPublisher` after mutating state

---

## Package Layout

```
codevaldcomm/              ← root package: public API (schema, type aliases, errors)
├── schema.go              ← DefaultCommSchema() — one source of truth for all types/relationships
├── models.go              ← CommDataManager, CommSchemaManager, CrossPublisher type aliases
├── errors.go              ← exported error sentinels
├── cmd/
│   ├── server/main.go     ← thin production entry point (env vars only)
│   └── dev/main.go        ← thin dev entry point (sets local defaults, then calls app.Run)
├── internal/
│   ├── app/app.go         ← shared runtime wiring (ArangoDB, seed, registrar, gRPC, cmux)
│   ├── config/config.go   ← Config struct loaded from env vars
│   ├── httphandler/       ← HTTP convenience handlers (9 communication flows)
│   ├── registrar/         ← CodeValdCross registration + CrossPublisher
│   └── server/            ← gRPC CommService + EntityService wrappers
├── storage/arangodb/      ← thin Backend alias over SharedLib's ArangoDB implementation
└── gen/go/codevaldcomm/v1/ ← generated proto stubs (do not edit)
```

---

## Schema Rules

**`DefaultCommSchema()` is the single source of truth.** Never hardcode type IDs or collection
names elsewhere.

```go
// ✅ CORRECT — reference via schema constants or DefaultCommSchema()
schema := codevaldcomm.DefaultCommSchema()

// ❌ WRONG — hardcoding a collection name in handler code
col := "comm_messages"
```

Every entity field must be a typed `PropertyDefinition` — no freeform `attributes` maps.

---

## HTTP Handler Rules

- Route by splitting the path with `strings.SplitN(path, "/", 4)` — no third-party router
- Each handler reads its agency ID from the URL and delegates to `CommDataManager`
- Publish domain events via `CrossPublisher` **after** a successful mutation, never before
- Edit-window enforcement is the handler's responsibility, not the storage layer's

```go
// ✅ CORRECT — check edit window before updating
closed, err := h.checkEditWindow(channel, msg)
if err != nil { ... }
if closed { respondError(w, http.StatusForbidden, codevaldcomm.ErrEditWindowClosed.Error()); return }
```

---

## gRPC Server Rules

- `CommServer` handles only `GetSchema` — delegates to `SchemaManager.GetActive`
- All entity RPCs are served by `entityserver.EntityServer` from SharedLib (registered separately)
- Map domain errors to gRPC status codes in `internal/server/errors.go`

---

## Error Handling Rules

- Return typed sentinels from `errors.go` at domain boundaries
- Map to gRPC status codes in `internal/server/errors.go`; HTTP handlers use `respondError`
- Never use `log.Fatal` inside library or handler code — return errors to `app.Run`

```go
// errors.go
var ErrEditWindowClosed = errors.New("edit window closed")
var ErrInvalidEntity     = errors.New("invalid entity")
```

---

## Context Rules

Every exported method must accept `context.Context` as its first argument. Respect cancellation
in loops and long-running operations.

---

## Task Management and Workflow

### Branch Management (MANDATORY)

```bash
# Create feature branch from main
git checkout -b feature/COMM-XXX_description

# Implement and validate
go build ./...           # must succeed
go test -v -race ./...   # must pass
go vet ./...             # must show 0 issues
golangci-lint run ./...  # must pass

# Merge when complete
git checkout main
git merge feature/COMM-XXX_description --no-ff
git branch -d feature/COMM-XXX_description
```

### Pre-Development Checklist

Before adding new code:
1. ✅ Is this type already defined in `schema.go`, `models.go`, or `errors.go`?
2. ✅ Am I delegating entity/relationship CRUD to `CommDataManager` (not writing ArangoDB queries)?
3. ✅ Does every new handler publish a domain event via `CrossPublisher` after mutation?
4. ✅ Does this function accept `context.Context` as its first argument?
5. ✅ Will the file exceed 500 lines after this change?
6. ✅ Are gRPC errors mapped through `internal/server/errors.go`?

### Code Review Requirements

Every PR must verify:
- [ ] No ArangoDB queries outside `storage/arangodb/`
- [ ] All entity mutations go through `CommDataManager`
- [ ] HTTP handlers publish events after successful mutations
- [ ] Edit-window logic is enforced before any message update
- [ ] Context propagated through all public calls
- [ ] No files exceeding 500 lines
- [ ] Tests added for new handlers and gRPC methods
- [ ] `go vet ./...` shows 0 issues
- [ ] `go test -race ./...` passes
