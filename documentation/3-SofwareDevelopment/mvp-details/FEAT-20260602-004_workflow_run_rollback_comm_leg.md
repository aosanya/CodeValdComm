# FEAT-20260602-004 — CodeValdComm leg of WorkflowRun rollback (`RollbackByWorkflowRun`)

**Status:** ✅ Shipped — CodeValdComm's portion of [CodeValdWork FEAT-20260602-004](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-004_workflow_run_rollback_semantics.md) Phase 2
**Owner:** CodeValdComm
**Branch:** `feature/Dev-COMM-FEAT-20260602-004_workflow-run-rollback-leg`

---

## Overview

When CodeValdWork's rollback coordinator runs (`POST /workflow-runs/{id}/rollback`), it calls every downstream service to compensate the artifacts that service produced under the rolled-back run. The Comm rule from the FEAT spec:

> **CodeValdComm** — Send a "this pipeline was rolled back" follow-up message into the same conversation. Don't delete prior messages (audit + recipients may have already read them).

Comm differs from the AI / Functions legs in that there is no in-flight "Message" state to cancel — Messages are point-in-time events. Compensation is therefore additive: a single synthetic notification Message is appended per affected channel, and the original pipeline-generated Messages are preserved.

---

## API

### gRPC

`CommService.RollbackByWorkflowRun(RollbackByWorkflowRunRequest) → RollbackByWorkflowRunResponse`

Request:

| Field | Type | Required | Description |
|---|---|---|---|
| `agency_id` | string | yes | Comm scopes every entity by agency. |
| `workflow_run_id` | string | yes | The `WorkflowRun.ID` whose Messages must be compensated. Empty → `INVALID_ARGUMENT`. |
| `reason` | string | no | Operator rollback reason. Recorded on the synthetic notification Message (`rollback_reason` property) and forwarded in the `comm.pipeline.rolled_back` event payload. |

Response:

| Field | Type | Description |
|---|---|---|
| `workflow_run_id` | string | Echoes the request. |
| `notified_channel_ids` | repeated string | Channels that received a fresh rollback notification Message during this call. |
| `skipped_channel_ids` | repeated string | Channels that already held a rollback notification for this run; included so the call is observably idempotent. |
| `notification_message_ids` | repeated string | The Message IDs created during this call, aligned 1:1 with `notified_channel_ids`. |

### gRPC status mapping

| Domain error | gRPC code |
|---|---|
| `ErrWorkflowRunIDRequired` | `INVALID_ARGUMENT` |
| `entitygraph` storage error | `INTERNAL` |

---

## Per-channel rule

A channel is considered "in the closure" of the rolled-back run when it owns at least one `Message` whose `workflow_run_id` property matches the rolled-back ID. The owning channel is resolved via the existing `Channel ──has_message──► Message` edge.

For each such channel:

| Channel state before call | Action | Result bucket | Event |
|---|---|---|---|
| Has at least one matching Message, no existing rollback notification for this run | post one synthetic Message | `notified_channel_ids` | `comm.pipeline.rolled_back` |
| Already holds a Message with `rollback_notification == true` and matching `workflow_run_id` | skip | `skipped_channel_ids` | — |

Orphan messages (a Message tagged with the run but with no inbound `has_message` edge) are ignored; there is no channel to notify into.

---

## Data model additions

### `Message` schema properties (added to `DefaultCommSchema()`)

```go
// rollback_notification is true on the synthetic "pipeline rolled back"
// follow-up message posted by RollbackByWorkflowRun.
{Name: "rollback_notification", Type: types.PropertyTypeBoolean}
// rollback_reason carries the operator-supplied reason string on a
// rollback notification message. Empty on regular messages.
{Name: "rollback_reason", Type: types.PropertyTypeString}
```

### Synthetic notification Message — properties set

| Property | Value |
|---|---|
| `body` | `"Pipeline {workflow_run_id} was rolled back[: {reason}]"` |
| `sender_id` | `codevaldcomm.RollbackSenderID` (constant `"codevald.rollback"`) — lets frontends render a system badge. |
| `workflow_run_id` | The rolled-back run ID, so the notification is itself discoverable via `GET /messages?workflow_run_id=X`. |
| `rollback_notification` | `true` |
| `rollback_reason` | The request `reason`, omitted when empty. |
| `created_at` / `updated_at` | Current UTC time, RFC3339. |

The notification is linked to its channel via a `has_message` edge, identical in shape to the edge produced by `SendMessage`.

---

## Events

### `comm.pipeline.rolled_back`

Published once per channel notified during this call (skipped channels do not produce an event).

```go
type PipelineRolledBackPayload struct {
    ChannelID     string
    MessageID     string
    WorkflowRunID string
    Reason        string
}
```

Publish failures are routed through `eventbus.SafePublish` — per the platform rule, a publish error must never fail the domain mutation.

---

## Idempotency

The call is idempotent at the per-channel level: a second call after a successful one finds each previously notified channel already holding a `rollback_notification == true` Message for the run, and routes it into `skipped_channel_ids` without re-posting or re-publishing.

This matches the coordinator's retry-after-`rollback_failed` path. If Phase 2 succeeded for Comm but failed for another service, re-running the rollback re-issues the Comm call without producing duplicate notifications.

---

## Implementation

| Concern | File |
|---|---|
| Domain function `RollbackByWorkflowRun(ctx, dm, pub, agencyID, workflowRunID, reason) (Result, error)` | [`workflow_run_rollback.go`](../../../workflow_run_rollback.go) |
| Topic + payload | [`events.go`](../../../events.go) |
| Sentinel error `ErrWorkflowRunIDRequired` | [`errors.go`](../../../errors.go) |
| Schema property additions | [`schema.go`](../../../schema.go) |
| Proto contract | [`proto/codevaldcomm/v1/comm.proto`](../../../proto/codevaldcomm/v1/comm.proto) |
| gRPC handler | [`internal/server/server.go`](../../../internal/server/server.go) |
| gRPC error mapping | [`internal/server/errors.go`](../../../internal/server/errors.go) |
| Wiring (DataManager + publisher) | [`internal/app/app.go`](../../../internal/app/app.go) |

---

## Tests

| Layer | File |
|---|---|
| Domain (`RollbackByWorkflowRun` function) | [`workflow_run_rollback_test.go`](../../../workflow_run_rollback_test.go) |
| gRPC handler + status converter | [`internal/server/workflow_run_rollback_server_test.go`](../../../internal/server/workflow_run_rollback_server_test.go) |

Covered scenarios:

- Empty `workflow_run_id` → `ErrWorkflowRunIDRequired` / `INVALID_ARGUMENT`.
- No matching messages → empty result, no events, no entities created.
- Single channel with multiple matching messages → one notification, one event.
- Multiple channels in the closure → one notification per channel; unrelated runs in other channels are not touched.
- Already-notified channel → skipped, no event, no second notification.
- Idempotent retry — second call returns the previously-notified channel in `skipped_channel_ids` and emits no event.
- Orphan message (no owning channel) → ignored without crashing.
- Nil publisher → notifications still posted, no panic.
- Storage error from `ListEntities` → propagated to the caller (RPC layer maps to `INTERNAL`).
- `AllTopics()` includes `comm.pipeline.rolled_back`.

---

## Open follow-ups

1. **CodeValdWork coordinator wiring** — `stubCompensateComm` in [`CodeValdWork/workflow_run_rollback.go`](../../../../CodeValdWork/workflow_run_rollback.go) still logs and returns. Replace with a gRPC call to `RollbackByWorkflowRun` so the end-to-end coordinator actually invokes the Comm leg.
2. **Silent-rollback flag** — the umbrella FEAT recommends a `silent: true` request flag for pipelines that should not notify on rollback (e.g. silent rollback of a failed scenario test). Not added yet — wait for the operator UI in the closure view to decide what the toggle looks like.
3. **Frontend surfacing** — the WorkFrontend run-detail view and Comm conversation views need to render the synthetic rollback message distinctly (system badge, distinct chip).
