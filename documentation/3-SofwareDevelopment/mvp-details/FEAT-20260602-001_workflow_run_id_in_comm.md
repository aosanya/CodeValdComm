# FEAT-20260602-001 — `workflow_run_id` propagation in CodeValdComm

**Status:** 📋 Not Started
**Severity:** Medium — sibling of the umbrella; messages aren't always pipeline-driven (some are operator-initiated chat), so the field is often empty — but when pipelines send messages (e.g. "merge failed, see this diagnostic") the link to the run is essential for the closure view
**Owner:** CodeValdComm
**Estimated effort:** ~1 day (schema + Message proto + handler propagation + list filter + integration tests)
**Source finding:** This conversation (2026-06-02) — sibling of [umbrella FEAT-20260602-001 in Cross](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md)

---

## Problem

CodeValdComm creates `Message` entities when pipelines emit notifications (e.g. diagnostic summaries from `merge-failure-diagnostics`, status updates from comm-routed plans). Today these messages have no link to the originating `WorkflowRun`, so the closure view can't show "what did this run say to whom."

## Goal

Make `workflow_run_id` a first-class typed field on:

- `Message` entity
- Every `comm.message.*` event payload (`comm.message.sent`, `comm.message.delivered`, `comm.message.failed`)
- `ListMessages` RPC / `GET /comm/{agencyId}/messages?workflow_run_id=X` filter

## Non-goals

- Adding `workflow_run_id` to `Conversation` or `Channel` config entities.
- Backfilling existing messages.

---

## Design

### Schema change

In `schema.go`, under the `Message` `TypeDefinition`:

```go
{Name: "workflow_run_id", Type: types.PropertyTypeString},
```

### Proto change

In `proto/codevaldcomm/v1/`:

- `Message` message: `string workflow_run_id = N;`
- `SendMessageRequest` accepts `string workflow_run_id` (optional — empty for operator-initiated chats).
- `ListMessagesRequest` accepts `string workflow_run_id` filter.

### Event payload changes

Every event emitted by Comm gains `workflow_run_id`. Read from inbound trigger event when the message is pipeline-driven; leave empty when the message originates from an interactive user.

### Chain-through behaviour

| Operation | Triggered by | Reads `workflow_run_id` from |
|---|---|---|
| Pipeline status message | An AI run that decides to send a message (via Comm API) | inbound event / API param |
| Diagnostic summary | `merge-failure-diagnostics` AgentRun completes and produces a comm-routed action | parent AgentRun.workflow_run_id |
| Operator chat | Interactive user message | empty (no pipeline context) |

---

## Implementation plan

### Phase 1 — Schema + proto (~0.5 day)

1. Add property to `Message` in `schema.go`.
2. Add proto fields; `make proto`.

### Phase 2 — Handlers + events (~0.25 day)

1. Update `SendMessage` to read + persist + propagate.
2. Update event payloads.

### Phase 3 — Tests (~0.25 day)

- Unit: send with run-id → persists; without → empty.
- Integration: AgentRun's diagnostic message carries the parent run-id.

---

## Verification

- `go test -race -count=1 ./...` clean.
- Run scenario 09 with a forced merge failure; `GET /comm/utility-app-builder/messages?workflow_run_id=$RUN` returns the diagnostic message produced by `merge-failure-diagnostics`.

---

## Open design questions

1. **Conversation grouping.** A long-running pipeline might send several messages in the same conversation. Do we group by `workflow_run_id` in the conversation view too? Recommend yes for the closure UI; defer for the main Comm UI.
2. **Operator replies.** If a user replies to a pipeline-generated message, does the reply inherit `workflow_run_id`? Recommend no — it becomes an operator action; pipelines own only what they produce.

---

## Dependencies

- Part of umbrella: [FEAT-20260602-001 in Cross](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md).
- Pairs with: [AI sibling FEAT](../../../../CodeValdAI/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_in_ai.md) — AI runs are the most common caller of `SendMessage` from within a pipeline.
