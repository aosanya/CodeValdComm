# MVP Done — Completed Tasks

Completed tasks are removed from `mvp.md` and recorded here with their completion date.

| Task ID | Title | Completion Date | Branch | Coding Session |
|---------|-------|-----------------|--------|----------------|
| MVP-COMM-001 | Module Scaffolding | 2026-05-11 | main | `go.mod`, `errors.go`, `models.go`, `schema.go` skeleton, `internal/config`, `cmd/main.go` + `cmd/dev` |
| MVP-COMM-002 | Pre-delivered Schema | 2026-05-11 | main | `DefaultCommSchema()` — 5 TypeDefinitions + 7 RelationshipDefinitions; `schema_test.go` |
| MVP-COMM-003 | ArangoDB Backend | 2026-05-11 | main | `storage/arangodb/backend.go` + `storage/codevaldcomm/backend.go`; edge collection `comm_relationships`; idempotent bootstrap |
| MVP-COMM-004 | gRPC Proto & Codegen | 2026-05-11 | main | `proto/codevaldcomm/v1/comm.proto` — `CommService.GetSchema`; `buf generate` → `gen/go/codevaldcomm/v1/` |
| MVP-COMM-005 | gRPC Server Implementation | 2026-05-11 | main | `internal/server/server.go`, `entity_server.go`, `errors.go`; `toGRPCError` mapping |
| MVP-COMM-006 | HTTP Convenience Handlers | 2026-05-11 | main | `internal/httphandler/handler.go` — all 9 flows; `topics.go` topic constants |
| MVP-COMM-007 | CodeValdCross Registration | 2026-05-11 | main | `internal/registrar/registrar.go`; `cmd/server/main.go` fully wired; heartbeat 20 s |
| MVP-COMM-008 | Unit & Integration Tests | 2026-05-11 | main | `schema_test.go`, `internal/server/server_test.go`, `internal/httphandler/handler_test.go`; all tests pass |
| FEAT-20260602-001 | workflow_run_id on Message + comm.* event payloads | 2026-06-02 | main | `schema.go` +property; `events.go` new payloads/topics; handler split → 4 files; new GET /messages?workflow_run_id=X; 4 new tests |
| FEAT-20260602-004 | CodeValdComm leg of WorkflowRun rollback (`RollbackByWorkflowRun` RPC) | 2026-06-02 | feature/Dev-COMM-FEAT-20260602-004_workflow-run-rollback-leg | proto: `RollbackByWorkflowRun` RPC + req/resp; `schema.go`: `rollback_notification` + `rollback_reason` on Message; `events.go`: `TopicPipelineRolledBack` + `PipelineRolledBackPayload`; `errors.go`: `ErrWorkflowRunIDRequired`; `workflow_run_rollback.go`: domain function — find channels owning matching messages, post one synthetic system message per channel, skip already-notified channels (idempotent); `internal/server/server.go`: handler + new CommServer constructor (`dm`, `sm`, `pub`); `internal/server/errors.go`: `ErrWorkflowRunIDRequired` → INVALID_ARGUMENT; `internal/app/app.go`: wire dm + pub; 9 domain tests + 3 RPC tests; `go build` ✅ `go vet` ✅ `go test -race ./...` ✅ |
