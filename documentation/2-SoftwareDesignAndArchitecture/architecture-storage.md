# CodeValdComm — Architecture: Storage

> Part of [architecture.md](architecture.md)

## 1. Database

CodeValdComm stores all data in the existing shared ArangoDB database
`codevald_demo`. All collection names carry the `comm_` prefix to avoid
collision with other services in the same database.

---

## 2. Collection Inventory

| Collection | Type | Holds |
|---|---|---|
| `comm_schemas` | Document | Pre-delivered Schema documents per agency |
| `comm_groups` | Document | Channel entities |
| `comm_messages` | Document | Message and thread-reply entities |
| `comm_participants` | Document | Participant vertex entities (comm-domain user projection) |
| `comm_edit_history` | Document | Immutable EditHistory entities |
| `comm_attachments` | Document | Immutable Attachment entities |
| `comm_relationships` | Edge | All relationship edges (`has_member`, `has_message`, `is_reply_to`, `has_attachment`, `has_reaction`, `read_by`, `has_edit`) |

---

## 3. Named Graph

```
Graph name:  comm_graph
Edge collection:     comm_relationships
Vertex collections:  comm_groups
                     comm_messages
                     comm_participants
                     comm_edit_history
                     comm_attachments
```

All `TraverseGraph` calls issued by the entity-graph engine use `comm_graph`
as the named graph. The `_from` and `_to` fields in `comm_relationships` edge
documents reference keys within these five vertex collections.

---

## 4. Document Shapes

### comm_schemas

```json
{
  "_key":     "<agencyID>",
  "agencyID": "agency-001",
  "version":  1,
  "types": [
    {
      "name":              "Channel",
      "displayName":       "Channel",
      "storageCollection": "comm_groups",
      "immutable":         false,
      "properties": [...]
    }
  ],
  "seededAt": "2025-01-01T00:00:00Z"
}
```

One document per agency. Keyed by `agencyID`. The schema is seeded exactly
once; re-seeding is a no-op (check existence before insert).

---

### comm_groups (Channel)

```json
{
  "_key":               "<uuid>",
  "agencyID":           "agency-001",
  "typeID":             "Channel",
  "name":               "general",
  "description":        "Team-wide announcements",
  "isDirect":           false,
  "editWindowSeconds":  300,
  "createdBy":          "<participantID>",
  "isArchived":         false,
  "createdAt":          "2025-01-01T12:00:00Z",
  "updatedAt":          "2025-01-01T12:00:00Z"
}
```

For Direct Message channels: `isDirect: true`, name is omitted or set to
the two participant IDs joined by `":"`. Exactly two `has_member` edges exist.

---

### comm_messages (Message)

```json
{
  "_key":         "<uuid>",
  "agencyID":     "agency-001",
  "typeID":       "Message",
  "content":      "Hello, world!",
  "authorID":     "<participantID>",
  "isThreadRoot": false,
  "editCount":    0,
  "editedAt":     null,
  "createdAt":    "2025-01-01T12:01:00Z",
  "updatedAt":    "2025-01-01T12:01:00Z"
}
```

Thread root messages: `isThreadRoot: true`. Replies point to them via
`is_reply_to` edges (source = reply, target = root). The thread root itself
remains in the channel via its `has_message` edge.

---

### comm_participants (Participant)

```json
{
  "_key":        "<uuid>",
  "agencyID":    "agency-001",
  "typeID":      "Participant",
  "userID":      "external-user-id",
  "displayName": "Alice",
  "avatarURL":   "https://...",
  "presence":    "online",
  "createdAt":   "2025-01-01T10:00:00Z",
  "updatedAt":   "2025-01-01T12:05:00Z"
}
```

At most one Participant per `(agencyID, userID)` pair. Presence is updated
in-place via `UpdateEntity`.

---

### comm_edit_history (EditHistory)

```json
{
  "_key":            "<uuid>",
  "agencyID":        "agency-001",
  "typeID":          "EditHistory",
  "previousContent": "Hello, worlf!",
  "editedBy":        "<participantID>",
  "version":         1,
  "editedAt":        "2025-01-01T12:10:00Z",
  "createdAt":       "2025-01-01T12:10:00Z"
}
```

Immutable — no `UpdateEntity` path exists for this type (`Immutable: true`).

---

### comm_attachments (Attachment)

```json
{
  "_key":      "<uuid>",
  "agencyID":  "agency-001",
  "typeID":    "Attachment",
  "fileName":  "report.pdf",
  "mimeType":  "application/pdf",
  "url":       "https://storage.example.com/report.pdf",
  "sizeBytes": 204800,
  "createdAt": "2025-01-01T12:02:00Z"
}
```

Immutable — `Immutable: true` in the TypeDefinition.

---

### comm_relationships (all edges)

```json
{
  "_key":   "<uuid>",
  "_from":  "comm_groups/<channelID>",
  "_to":    "comm_participants/<participantID>",
  "label":  "has_member",
  "agencyID": "agency-001",

  // label-specific properties:
  "role":     "member",
  "joinedAt": "2025-01-01T10:05:00Z",
  "invitedBy": "<participantID>"
}
```

Label-specific shapes:

| Label | Extra properties |
|---|---|
| `has_member` | `role`, `joinedAt`, `invitedBy` |
| `has_message` | `postedAt` |
| `is_reply_to` | — |
| `has_attachment` | `attachedAt` |
| `has_reaction` | `emoji`, `reactedAt` |
| `read_by` | `readAt` |
| `has_edit` | `editedAt` |

---

## 5. Indexes

### comm_groups
| Fields | Type | Purpose |
|---|---|---|
| `agencyID` | persistent | All channel queries are scoped by agency |
| `agencyID, isDirect` | persistent | List DM channels for an agency |

### comm_messages
| Fields | Type | Purpose |
|---|---|---|
| `agencyID` | persistent | All message queries scoped by agency |
| `agencyID, authorID` | persistent | Messages by author |
| `createdAt` | persistent, ascending | Chronological message ordering |

### comm_participants
| Fields | Type | Purpose |
|---|---|---|
| `agencyID` | persistent | Scope participant queries |
| `agencyID, userID` | persistent, unique | Enforce one Participant per user per agency |
| `presence` | persistent | Presence queries (online users) |

### comm_edit_history
| Fields | Type | Purpose |
|---|---|---|
| `agencyID` | persistent | Scope history queries |
| `editedAt` | persistent, ascending | Chronological edit ordering |

### comm_attachments
| Fields | Type | Purpose |
|---|---|---|
| `agencyID` | persistent | Scope attachment queries |

### comm_relationships
| Fields | Type | Purpose |
|---|---|---|
| `_from, label` | persistent | Outbound edge traversal by label |
| `_to, label` | persistent | Inbound edge traversal by label |
| `agencyID` | persistent | Scope all edge queries |
| `agencyID, label` | persistent | Cross-agency edge isolation |
