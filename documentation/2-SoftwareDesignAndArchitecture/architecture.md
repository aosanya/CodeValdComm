# CodeValdComm — Architecture

## Contents

| File | Covers |
|---|---|
| [architecture-domain.md](architecture-domain.md) | Pre-delivered schema · TypeDefinitions · Graph relationship model |
| [architecture-storage.md](architecture-storage.md) | ArangoDB collections · Document shapes · Indexes · Named graph |
| [architecture-service.md](architecture-service.md) | Package structure · gRPC service · Convenience HTTP routes · Cross registration |
| [architecture-flows.md](architecture-flows.md) | Critical path flows: send message · promote thread · edit message · reaction · read receipt · presence |

## Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Entity-graph foundation | `CommDataManager = entitygraph.DataManager` (SharedLib) | Same infrastructure as CodeValdDT; agencies + graph ops for free |
| Pre-delivered schema | Fixed TypeDefinitions bundled in the service; seeded on first run per agency | Comm domain is well-known; no custom type authoring needed |
| Thread model | `isThreadRoot: bool` flag on Message; replies anchor via `is_reply_to` edge | Simpler than a separate Thread entity; graph traversal gives full thread tree |
| Participant as entity | `Participant` is a vertex in `comm_participants` | External user IDs are projected into the Comm graph so edges are valid ArangoDB graph edges |
| Relationships as edges | `has_member`, `has_message`, `is_reply_to`, `has_reaction`, `read_by`, `has_attachment`, `has_edit` in `comm_relationships` | Graph-first; enables AQL traversal across the full conversation graph |
| Edit history | Immutable `EditHistory` entity per edit, linked via `has_edit` edge | Full audit trail; visible to participants; edit window controlled by channel-level config |
| Direct messages | A `Channel` with `isDirect: true` and exactly 2 `has_member` edges | Reuses Channel/Message/Participant model; no separate DM entity needed |
| Convenience routes | Thin HTTP routes registered with Cross; each maps to one or two entity-graph operations | Domain-semantic API without duplicating business logic |
| Storage isolation | One collection per entity type (`comm_groups`, `comm_messages`, …) | Query efficiency; clear ownership; matches DT `dt_` prefix pattern |
| Pub/sub | Type-aware topics (`comm.message.sent`, etc.) | Consumers care about semantic events, not raw entity creates |
| Database | Pre-existing shared `codevald_demo`; `comm_` prefixed collections | Same pattern as CodeValdDT |
