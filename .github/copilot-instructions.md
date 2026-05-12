````instructions
# CodeValdComm — AI Agent Development Instructions

## Project Overview

**CodeValdComm** is a **Go gRPC microservice** that manages the communication
layer of the CodeVald platform — channels, messages, threads, reactions, read
receipts, attachments, participants, and presence.

Unlike CodeValdDT (where agencies author their own entity types), CodeValdComm
ships with a **pre-delivered, fixed schema**. TypeDefinitions for `Channel`,
`Message`, `Participant`, `EditHistory`, and `Attachment` are baked into
`schema.go` and seeded per agency on first use (idempotent).

**Core Concept**: CodeValdComm is a thin orchestration layer over
`entitygraph.DataManager` from CodeValdSharedLib. Every domain operation maps
to one or two entity-graph calls. All HTTP convenience routes delegate to
`CommDataManager`; no business logic lives in handlers.

**Current status**: `SHAREDLIB-010` (`entitygraph` package) is ✅ complete —
all COMM tasks are **unblocked**. No Go code scaffolded yet.
Start with `MVP-COMM-001` (module scaffolding).

---

## Service Architecture

```
CodeValdHi / gRPC callers
        │  HTTP :8080 (proxied via Cross)   gRPC :50060
        ▼                                         ▼
  internal/httphandler/handler.go    internal/server/server.go
        │                                         │
        └─────────────┬───────────────────────────┘
                      ▼
              CommDataManager  (alias: entitygraph.DataManager from SharedLib)
              CommSchemaManager (alias: entitygraph.SchemaManager from SharedLib)
                      │                           │ pub/sub events
              storage/arangodb/backend.go         ▼
                      │               CodeValdCross gRPC bus (Publish RPC)
              ArangoDB `codevald_demo` DB
              (comm_ prefixed collections)
```

**Key invariants:**

- `CommDataManager` and `CommSchemaManager` are **always injected** — never constructed inside handlers or manager
- `comm_relationships` **must be created as an ArangoDB edge collection** — this cannot be changed after creation
- Schema is seeded into `comm_schemas` once per agency on `agency.created`; `SetSchema` is **not** exposed via gRPC
- Every mutating HTTP flow publishes a typed pub/sub event via the **CodeValdCross gRPC bus** — there is no in-process bus in this service
- All pub/sub topic strings are **constants** — never inline string literals or runtime concatenation
- No imports from CodeValdGit, CodeValdWork, CodeValdAgency, or CodeValdCross Go packages — gRPC only

---

## Project Layout

```
CodeValdComm/
├── cmd/main.go                      # Dependency wiring only
├── errors.go                        # ErrEntityNotFound, ErrEditWindowClosed, etc.
├── models.go                        # CommDataManager, CommSchemaManager type aliases
├── schema.go                        # defaultCommSchema (pre-delivered TypeDefinitions)
├── go.mod                           # module github.com/aosanya/CodeValdComm
├── internal/
│   ├── manager/manager.go           # Concrete CommDataManager implementation
│   ├── server/server.go             # gRPC CommService handlers (thin — delegate to manager)
│   ├── httphandler/handler.go       # HTTP convenience route handlers
│   ├── config/config.go             # Config struct + loader
│   └── registrar/registrar.go       # Cross heartbeat + schema seeding on agency.created
├── storage/arangodb/backend.go      # ArangoDB entitygraph.DataManager + SchemaManager impl
└── proto/codevaldcomm/v1/comm.proto
```

---

## Storage: ArangoDB Collections

| Collection | Type | Holds |
|---|---|---|
| `comm_schemas` | Document | Pre-delivered schema per agency (keyed by `agencyID`) |
| `comm_groups` | Document | Channel entities |
| `comm_messages` | Document | Message entities |
| `comm_participants` | Document | Participant vertex entities |
| `comm_edit_history` | Document | Immutable EditHistory entities |
| `comm_attachments` | Document | Immutable Attachment entities |
| `comm_relationships` | **Edge** | ALL relationship types (see below) |

Named graph: `comm_graph` (edge: `comm_relationships`; vertices: all 5 document collections).

Relationship labels: `has_member`, `has_message`, `is_reply_to`, `has_attachment`,
`has_reaction`, `read_by`, `has_edit`.

---

## Critical Flows

### SendMessage
`CreateEntity(Message)` → `CreateRelationship(has_message, Channel→Message)` → publish `TopicMessageSent` via Cross bus

### EditMessage
Fetch channel → check `editWindowSeconds` (`0`=closed, `-1`=always open, `>0`=window) → if closed return `ErrEditWindowClosed` → `CreateEntity(EditHistory)` → `CreateRelationship(has_edit)` → `UpdateEntity(Message)` → publish `TopicMessageEdited` via Cross bus

### PromoteToThread
`UpdateEntity(Message, isThreadRoot=true)` → publish `TopicThreadPromoted` via Cross bus. Idempotent if already a thread root.

### Thread replies
`CreateEntity(Message)` → `CreateRelationship(is_reply_to, Reply→Root)`. Traverse inbound `is_reply_to` for the full thread.

---

## Pub/Sub Events (must publish on every successful mutating flow)

All topic strings are **constants** defined in one file (e.g. `topics.go`). Publishing goes through the injected CodeValdCross gRPC bus client — not an in-process bus.

```go
// ✅ CORRECT — package-level constants, no agencyID in topic string
const (
    TopicMessageSent        = "comm.message.sent"
    TopicMessageEdited      = "comm.message.edited"
    TopicThreadPromoted     = "comm.thread.promoted"
    TopicReactionAdded      = "comm.reaction.added"
    TopicMemberJoined       = "comm.member.joined"
    TopicParticipantPresence = "comm.participant.presence"
)

// ❌ WRONG — cross. prefix and agencyID in topic string
topic := "cross.comm." + agencyID + ".message.sent"
```

| Topic Constant | Trigger |
|---|---|
| `TopicMessageSent` | SendMessage |
| `TopicMessageEdited` | EditMessage |
| `TopicThreadPromoted` | PromoteToThread |
| `TopicReactionAdded` | AddReaction |
| `TopicMemberJoined` | AddMember |
| `TopicParticipantPresence` | UpdatePresence |

**Consumes**: `agency.created` → seed `defaultCommSchema` for the new agency (idempotent).

---

## Cross Registration

```go
RegisterRequest{
    ServiceName: "codevaldcomm",
    Addr:        ":50060",
    // Produces and Consumes as listed above
    Routes: commRoutes(), // all /{agencyId}/comm/... HTTP convenience routes
}
```

Use `registrar` package from CodeValdSharedLib. Heartbeat every 20 seconds.

---

## Error Types (`errors.go`)

| Error | gRPC Code |
|---|---|
| `ErrEntityNotFound` | `codes.NotFound` |
| `ErrRelationshipNotFound` | `codes.NotFound` |
| `ErrSchemaNotFound` | `codes.NotFound` |
| `ErrInvalidEntity` | `codes.InvalidArgument` |
| `ErrInvalidRelationship` | `codes.InvalidArgument` |
| `ErrImmutableType` | `codes.FailedPrecondition` |
| `ErrEditWindowClosed` | `codes.FailedPrecondition` |

---

## Developer Workflows

```bash
# Build (library check)
go build ./...

# Test with race detector
go test -v -race ./...

# Static analysis
go vet ./...

# Lint
golangci-lint run ./...

# Proto regeneration
buf generate
```

### Branch naming

```bash
git checkout -b feature/COMM-XXX_description
# ... implement, validate, then:
git checkout main
git merge feature/COMM-XXX_description --no-ff
git branch -d feature/COMM-XXX_description
```

---

## Key Differences from CodeValdDT

| | CodeValdDT | CodeValdComm |
|---|---|---|
| Schema ownership | Agency authors their own TypeDefinitions | Fixed schema; seeded per agency from `defaultCommSchema` in `schema.go` |
| `SetSchema` via gRPC | ✅ exposed | ❌ not exposed — schema is internal only |
| Entity types | Agency-defined | Channel, Message, Participant, EditHistory, Attachment |
| Edit semantics | N/A | `editWindowSeconds` on Channel controls edit window |
| Threads | N/A | `isThreadRoot: bool` on Message; `is_reply_to` edges |

---

## SharedLib Dependencies

| Package | Used for |
|---|---|
| `entitygraph` | `DataManager` + `SchemaManager` interfaces (aliased as `CommDataManager`, `CommSchemaManager`) |
| `registrar` | Cross heartbeat + topic subscription |
| `serverutil` | `NewGRPCServer`, `RunWithGracefulShutdown` |
| `arangoutil` | `Connect(ctx, Config)` for ArangoDB |
| `types` | `TypeDefinition`, `Schema`, `PropertyDefinition`, `PathBinding` |

---

## Anti-Patterns

- ❌ **Business logic in gRPC handlers or HTTP handlers** — delegate to `CommDataManager`
- ❌ **Raw topic strings or runtime concatenation** — define constants in one file (e.g. `topics.go`)
- ❌ **In-process pub/sub bus** — events are published via the injected CodeValdCross gRPC client
- ❌ **Storing relationships in a document collection** — `comm_relationships` must be an edge collection
- ❌ **Exposing `SetSchema` via gRPC** — schema is pre-delivered and seeded internally
- ❌ **Skipping pub/sub events** — every mutating flow must publish its event
- ❌ **Direct imports of other CodeVald service packages** — gRPC only
- ❌ **Hardcoding ArangoDB connection or collection names in `internal/manager/`** — inject `Backend`

---

## Documentation References

- `documentation/2-SoftwareDesignAndArchitecture/architecture-domain.md` — TypeDefinitions, graph model
- `documentation/2-SoftwareDesignAndArchitecture/architecture-storage.md` — collection inventory, document shapes, indexes
- `documentation/2-SoftwareDesignAndArchitecture/architecture-service.md` — gRPC proto, HTTP routes, Cross registration, project layout
- `documentation/2-SoftwareDesignAndArchitecture/architecture-flows.md` — step-by-step logic for each HTTP flow (SendMessage, EditMessage, PromoteToThread, AddReaction, MarkRead, UpdatePresence)
- `documentation/3-SofwareDevelopment/mvp.md` — task list (MVP-COMM-001 through MVP-COMM-008)
````
