# 3 — Software Development

## Overview

This section tracks the development plan, MVP task breakdown, and implementation
details for CodeValdComm.

---

## Index

| Document | Description |
|---|---|
| [mvp.md](mvp.md) | Active MVP scope, task list, and completion status |
| [mvp_done.md](mvp_done.md) | Completed tasks with completion dates and branches |

---

## MVP Status

| Task ID | Title | Status |
|---|---|---|
| MVP-COMM-001 | Module Scaffolding | ⏸️ Blocked on SHAREDLIB-010 |
| MVP-COMM-002 | Pre-delivered Schema | ⏸️ Blocked |
| MVP-COMM-003 | ArangoDB Backend | ⏸️ Blocked |
| MVP-COMM-004 | gRPC Service Proto & Codegen | ⏸️ Blocked |
| MVP-COMM-005 | gRPC Server Implementation | ⏸️ Blocked |
| MVP-COMM-006 | HTTP Convenience Handlers | ⏸️ Blocked |
| MVP-COMM-007 | CodeValdCross Registration | ⏸️ Blocked |
| MVP-COMM-008 | Unit & Integration Tests | ⏸️ Blocked |

---

## Execution Order

```
SHAREDLIB-010 (unblocks all Comm work)
      ↓
MVP-COMM-001  ← Module scaffolding, go.mod, errors.go, models.go
      ↓
┌─────────────┬─────────────┬──────────────┐
MVP-COMM-002  MVP-COMM-003  MVP-COMM-004
(schema.go)   (ArangoDB)    (proto+codegen)
└─────────────┴─────────────┴──────────────┘
      ↓
MVP-COMM-005  ← gRPC server implementation
      ↓
MVP-COMM-006  ← HTTP convenience handlers
      ↓
┌─────────────┬──────────────┐
MVP-COMM-007  MVP-COMM-008
(Cross reg.)  (tests)
```
