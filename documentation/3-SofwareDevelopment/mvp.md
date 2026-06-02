# MVP — CodeValdComm

## Task Overview
- **Objective**: Deliver CodeValdComm as a standalone gRPC microservice that
  manages real-time communication for each Agency — channels, messages, threads,
  reactions, read receipts, and participants — backed by `entitygraph.DataManager`
  and `entitygraph.SchemaManager` from CodeValdSharedLib, with ArangoDB storage
  and a full suite of HTTP convenience routes proxied through CodeValdCross.
- **Success Criteria**: MVP-COMM-001 through MVP-COMM-008 implemented and tested;
  service compiles; all unit tests pass; `comm_relationships` created as an edge
  collection; pre-delivered schema seeded on `agency.created`; all
  convenience routes registered with CodeValdCross on startup.
- **Dependencies**: ~~`SHAREDLIB-010`~~ ✅  ~~`SHAREDLIB-011`~~ ✅  ~~EntityService gRPC (`GetRelationship` + `TraverseGraph`)~~ ✅ — all SharedLib prerequisites are **complete and on main**. All COMM tasks are unblocked.

### SharedLib Prerequisites (all ✅ on main)

| Prerequisite | What it provides |
|---|---|
| ~~SHAREDLIB-010~~ ✅ | `entitygraph.DataManager`, `entitygraph.SchemaManager`, `Entity`, `Relationship`, `CreateEntityRequest`, `UpdateEntityRequest`, `TraverseGraphRequest`, `TraverseGraphResult`, ArangoDB backend |
| ~~SHAREDLIB-011~~ ✅ | `types.RelationshipDefinition`, schema versioning, `schemaroutes.RoutesFromSchema`, `registrar.New` accepts `[]types.RouteInfo` |
| EntityService gRPC ✅ | `entitygraph/server.EntityServer` — full gRPC service with `CreateEntity`, `GetEntity`, `UpdateEntity`, `DeleteEntity`, `ListEntities`, `CreateRelationship`, `GetRelationship`, `DeleteRelationship`, `ListRelationships`, `TraverseGraph` |

### Implementation Approach (decided)

- **`CommService` proto** (`proto/codevaldcomm/v1/comm.proto`) exposes **only `GetSchema`** — all entity/relationship RPCs are served by `EntityService` from SharedLib (registered directly in `cmd/main.go`). This avoids duplicating 10+ RPCs in the proto.
- **`CrossPublisher` interface** defined in root package (`models.go`); implemented by `internal/registrar`. Used by `httphandler` to publish events.
- **Edit-window logic**: read `editWindowSeconds` from Channel properties; `0` = editing disabled; `-1` = always editable; `>0` = seconds since `message.createdAt`.
- **Schema seeding**: triggered on `agency.created` event AND on startup if `CODEVALDCOMM_AGENCY_ID` env var is set (idempotent via `SetSchema`).
- **Collection naming**: all comm-specific, prefixed `comm_` (see MVP-COMM-003).

### Current Branch Context

- CodeValdSharedLib: `main` (all prerequisites merged)
- CodeValdAI: `feature/AI-008_grpc_proto` (adjacent work)
- CodeValdComm: **no git repo yet** — `git init` + first commit as part of MVP-COMM-001

## Platform Documentation
- **Requirements**: [requirements.md](../1-SoftwareRequirements/requirements.md)
- **Architecture**: [architecture.md](../2-SoftwareDesignAndArchitecture/architecture.md)

## Documentation Structure
- **High-Level Overview**: This file (`mvp.md`) — task tables, priorities, dependencies
- **Completion Record**: `mvp_done.md` — completed tasks with dates and branches

---

## Workflow Integration

### Task Management Process
1. **Task Assignment**: Pick tasks based on priority (P0 first) and dependencies
2. **Implementation**: Update "Status" column as work progresses (Blocked → Not Started → In Progress → Complete)
3. **Completion Process** (MANDATORY):
   - Create detailed coding session document in `coding_sessions/` using format: `{TaskID}_{description}.md`
   - Add completed task to summary table in `mvp_done.md` with completion date
   - Remove completed task from this active `mvp.md` file
   - Update any dependent task references using notation: `~~MVP-COMM-XXX~~ ✅`
   - Merge feature branch to main
4. **Dependencies**: Ensure prerequisite tasks are completed before starting dependent work

### Branch Management (MANDATORY)
```bash
# Create feature branch
git checkout -b feature/COMM-XXX_description

# Work, build validation, test
go build ./...
go vet ./...
go test -v -race ./...

# Merge when complete and tested
git checkout main
git merge feature/COMM-XXX_description --no-ff
git branch -d feature/COMM-XXX_description
```

---

## Status Legend
- ✅ **Completed** — done, merged to main (see `mvp_done.md`)
- 🚀 **In Progress** — currently being worked on
- 📋 **Not Started** — ready to begin (dependencies met)
- ⏸️ **Blocked** — waiting on dependencies

---

## Outstanding feature work

| Task ID | Title | Status | Depends On |
|---|---|---|---|
| FEAT-20260602-001 | `workflow_run_id` on `Message` + every `comm.*` event payload (Comm sibling of the [Cross umbrella](../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md)) | ✅ Done | FEAT-20260602-001 in CodeValdFunctions (start-pipeline) |
| FEAT-20260602-004 | `DELETE /by-workflow-run/{id}` rollback leg — send "pipeline rolled back" follow-up message into the conversation; do not delete prior messages ([Work umbrella](../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-004_workflow_run_rollback_semantics.md)) | 🚀 In Progress | FEAT-20260602-001 |

See [mvp-details/FEAT-20260602-001_workflow_run_id_in_comm.md](mvp-details/FEAT-20260602-001_workflow_run_id_in_comm.md).

---

## P0: Foundation

| Task ID | Title | Status | Depends On | Notes |
|---|---|---|---|---|
| MVP-COMM-001 | Module Scaffolding | 📋 Not Started | ~~SHAREDLIB-010~~ ✅ | `go.mod` (`module github.com/aosanya/CodeValdComm`); `replace github.com/aosanya/CodeValdSharedLib => ../CodeValdSharedLib`; `errors.go` (re-export entitygraph errors + `ErrEditWindowClosed`, `ErrInvalidEntity`); `models.go` (`CommDataManager`/`CommSchemaManager` type aliases, `CrossPublisher` interface); `schema.go` skeleton; `internal/config/config.go`; `cmd/main.go` skeleton; `git init` + initial commit on `feature/COMM-001_module-scaffolding` |

---

## P1: Core Service

| Task ID | Title | Status | Depends On | Notes |
|---|---|---|---|---|
| MVP-COMM-002 | Pre-delivered Schema | 📋 Not Started | MVP-COMM-001 | `schema.go`: `DefaultCommSchema() types.Schema` — `types.Schema{ID: "comm-schema-v1", ...}` with 5 TypeDefinitions: `Channel` (StorageCollection: `comm_groups`), `Participant` (comm_participants), `Message` (comm_messages), `EditHistory` (comm_edit_history, Immutable: true), `Attachment` (comm_attachments, Immutable: true); RelationshipDefinitions: `has_member` (Channel→Participant, ToMany: true), `has_message` (Channel→Message, ToMany: true), `is_reply_to` (Message→Message, ToMany: false), `has_attachment` (Message→Attachment, ToMany: true), `has_reaction` (Message→Participant, ToMany: true), `read_by` (Message→Participant, ToMany: true), `has_edit` (Message→EditHistory, ToMany: true); `schema_test.go` verifies all TypeDefinitions present |
| MVP-COMM-003 | ArangoDB Backend | 📋 Not Started | MVP-COMM-001 | `storage/arangodb/backend.go`; thin wrapper over `entitygraph/storage/arangodb`; collection config: EntityCollection `comm_entities`, RelCollection `comm_relationships` (**edge** — must be created as edge collection), SchemasDraftCol `comm_schemas_draft`, SchemasPublishedCol `comm_schemas_published`, GraphName `comm_graph`; document collections: `comm_groups`, `comm_messages`, `comm_participants`, `comm_edit_history`, `comm_attachments`; `New(ctx, driver.Database) (entitygraph.DataManager, entitygraph.SchemaManager, error)` — bootstraps collections + named graph on startup (idempotent) |
| MVP-COMM-004 | gRPC Proto & Codegen | 📋 Not Started | MVP-COMM-001 | `proto/codevaldcomm/v1/comm.proto` — `CommService` with **only `GetSchema`** RPC (entity+relationship ops served by `EntityService` from SharedLib); `buf.yaml` + `buf.gen.yaml`; `buf generate` → `gen/go/codevaldcomm/v1/`; `GetSchemaRequest {agency_id}` → `Schema` (re-use SharedLib schema message or define in comm proto) |
| MVP-COMM-005 | gRPC Server Implementation | 📋 Not Started | MVP-COMM-001, MVP-COMM-003, MVP-COMM-004 | `internal/server/server.go` — `CommServer` struct implements `CommServiceServer`; `GetSchema` handler delegates to `CommSchemaManager.GetPublishedSchema`; `internal/server/entity_server.go` — re-export `entitygraph/server.NewEntityServer` for use in `cmd/main.go`; `internal/server/errors.go` — `toGRPCError` mapping: `ErrEntityNotFound`→`NotFound`, `ErrRelationshipNotFound`→`NotFound`, `ErrSchemaNotFound`→`NotFound`, `ErrInvalidEntity`→`InvalidArgument`, `ErrImmutableType`→`FailedPrecondition`, `ErrEditWindowClosed`→`FailedPrecondition`, unknown→`Internal` |
| MVP-COMM-006 | HTTP Convenience Handlers | 📋 Not Started | MVP-COMM-005 | `internal/httphandler/handler.go`; `Handler` receives `entitygraph.DataManager`, `entitygraph.SchemaManager`, `CrossPublisher`; 9 flows: **(1) SendMessage** POST `channels/{channelId}/messages` — CreateEntity(Message)+CreateRelationship(has_message)+Publish(TopicMessageSent); **(2) PromoteToThread** PUT `messages/{messageId}/promote` — UpdateEntity(isThreadRoot=true)+Publish(TopicThreadPromoted), idempotent; **(3) EditMessage** PUT `messages/{messageId}` — fetch Channel→check editWindowSeconds (0=closed/ErrEditWindowClosed, -1=always, >0=time window)→CreateEntity(EditHistory)+CreateRelationship(has_edit)+UpdateEntity(Message)+Publish(TopicMessageEdited); **(4) AddReaction** POST `messages/{messageId}/reactions` — CreateRelationship(has_reaction)+Publish(TopicReactionAdded); **(5) MarkRead** POST `messages/{messageId}/read` — idempotent CreateRelationship(read_by); **(6) UpdatePresence** PUT `participants/{participantId}` — UpdateEntity+Publish(TopicParticipantPresence); **(7) CreateChannel** POST `channels` — CreateEntity(Channel); **(8) JoinChannel** POST `channels/{channelId}/members` — CreateRelationship(has_member)+Publish(TopicMemberJoined); **(9) CreateDM** POST `direct` — idempotent CreateEntity(Channel isDirect=true)+2×CreateRelationship(has_member); topic constants defined in `internal/httphandler/topics.go` |
| MVP-COMM-007 | CodeValdCross Registration | 📋 Not Started | MVP-COMM-005, MVP-COMM-006 | `internal/registrar/registrar.go` — wraps `sharedlib/registrar`; implements `CrossPublisher`; subscribes to `agency.created` for schema seeding; heartbeat every 20 s; `cmd/main.go` fully wired: ArangoDB connect → DataManager/SchemaManager → EntityServer + CommServer → HTTP handler → registrar → `serverutil.RunWithGracefulShutdown`; env vars: `CODEVALDCOMM_GRPC_PORT` (required), `COMM_ARANGO_ENDPOINT`, `COMM_ARANGO_USER`, `COMM_ARANGO_PASSWORD`, `COMM_ARANGO_DATABASE`, `CROSS_GRPC_ADDR`, `COMM_GRPC_ADVERTISE_ADDR`, `CODEVALDCOMM_AGENCY_ID`, `CROSS_PING_INTERVAL`, `CROSS_PING_TIMEOUT`; Cross registration: produces `comm.message.sent/edited/thread.promoted/reaction.added/member.joined/participant.presence`, consumes `agency.created`; routes via `schemaroutes.RoutesFromSchema(DefaultCommSchema(), "/{agencyId}/comm", ...)` |

---

## P2: Quality

| Task ID | Title | Status | Depends On | Notes |
|---|---|---|---|---|
| MVP-COMM-008 | Unit & Integration Tests | 📋 Not Started | MVP-COMM-001, MVP-COMM-003, MVP-COMM-005 | Table-driven unit tests with mock `CommDataManager` for gRPC handlers and HTTP handlers; `schema_test.go` — verify all 5 TypeDefinitions + 7 RelationshipDefinitions present in `DefaultCommSchema()`; flow tests for SendMessage, EditMessage (edit-window: closed/open/timed-out), PromoteToThread (idempotent); ArangoDB integration tests tagged `//go:build integration`; coverage ≥ 80% on `internal/server/` and `internal/httphandler/` |

---

## Success Criteria

- [ ] `go build ./...` succeeds
- [ ] `go test -race ./...` all pass
- [ ] `go vet ./...` shows 0 issues
- [ ] All `CommService` gRPC RPCs work end-to-end with ArangoDB
- [ ] `comm_relationships` created as **edge collection** (cannot be changed post-creation)
- [ ] `comm_graph` named graph bootstrapped on startup (idempotent)
- [ ] Pre-delivered schema seeded into `comm_schemas` on `agency.created` (idempotent)
- [ ] All HTTP convenience routes work end-to-end and are registered with CodeValdCross
- [ ] `comm.message.sent` published on every successful SendMessage
- [ ] `comm.message.edited` published on every successful EditMessage
- [ ] `comm.thread.promoted` published on every successful PromoteToThread
- [ ] `comm.reaction.added` published on every successful AddReaction
- [ ] Edit-window enforcement returns `ErrEditWindowClosed` (gRPC: `FAILED_PRECONDITION`) correctly
- [ ] CodeValdCross registration fires on startup and repeats every 20 seconds
- [ ] `CommDataManager` and `CommSchemaManager` injected via constructor — never hardcoded
- [ ] No direct imports of other CodeVald services
