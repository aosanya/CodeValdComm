# CodeValdComm — Architecture: Domain Model

> Part of [architecture.md](architecture.md)

## 1. Pre-Delivered Schema

CodeValdComm ships with a **fixed, built-in schema** — agencies do not author
TypeDefinitions. On first `CreateEntity` call for an agency, CodeValdComm
seeds the default schema into `comm_schemas` if none exists. Subsequent
calls reuse it; schema seeding is idempotent.

This contrasts with CodeValdDT, where agencies design their own entity types.
CodeValdComm's TypeDefinitions are a platform constant.

---

## 2. Entity TypeDefinitions

All TypeDefinitions below form the seeded `Schema` stored in `comm_schemas`.
Each specifies `StorageCollection` so the entity-graph engine routes writes
to the correct collection automatically.

### Channel

```
TypeDefinition{
    Name:              "Channel",
    DisplayName:       "Channel",
    StorageCollection: "comm_groups",
    Immutable:         false,
    Properties: [
        { Name: "name",              Type: string,   Required: true  },
        { Name: "description",       Type: string,   Required: false },
        { Name: "isDirect",          Type: boolean,  Required: false }, // true for DM channels
        { Name: "editWindowSeconds", Type: integer,  Required: false }, // 0 = no editing; -1 = always editable
        { Name: "createdBy",         Type: string,   Required: true  }, // participantID
        { Name: "isArchived",        Type: boolean,  Required: false },
    ],
}
```

A **Direct Message channel** is a Channel with `isDirect: true` and exactly
two `has_member` edges. The convenience route `POST /{agencyId}/comm/direct`
creates this automatically.

---

### Participant

```
TypeDefinition{
    Name:              "Participant",
    DisplayName:       "Participant",
    StorageCollection: "comm_participants",
    Immutable:         false,
    Properties: [
        { Name: "userID",      Type: string,  Required: true  }, // external user ID
        { Name: "displayName", Type: string,  Required: false },
        { Name: "avatarURL",   Type: string,  Required: false },
        { Name: "presence",    Type: option,  Required: false }, // "online" | "away" | "offline"
    ],
}
```

`Participant` is the Comm-domain projection of an external user. It becomes a
**graph vertex** so that `has_member`, `has_reaction`, and `read_by` edges can
reference it directly in ArangoDB. There is at most one Participant per
`(agencyID, userID)` pair.

---

### Message

```
TypeDefinition{
    Name:              "Message",
    DisplayName:       "Message",
    StorageCollection: "comm_messages",
    Immutable:         false,
    Properties: [
        { Name: "content",      Type: string,   Required: true  },
        { Name: "authorID",     Type: string,   Required: true  }, // participantID
        { Name: "isThreadRoot", Type: boolean,  Required: false }, // set to true by admin promote
        { Name: "editCount",    Type: integer,  Required: false }, // incremented on every edit
        { Name: "editedAt",     Type: datetime, Required: false }, // null if never edited
    ],
}
```

A message is **editable** within the `editWindowSeconds` configured on its
parent Channel. Each edit appends an immutable `EditHistory` entity and a
`has_edit` edge.

Setting `isThreadRoot: true` via `PUT /{agencyId}/comm/messages/{messageId}/promote`
promotes the message. Thread replies are posted as new Messages anchored via
`is_reply_to` edges. From the channel view, a thread-rooted message shows a
preview of thread activity; zooming in shows the full `is_reply_to` subgraph.

---

### EditHistory

```
TypeDefinition{
    Name:              "EditHistory",
    DisplayName:       "Edit History",
    StorageCollection: "comm_edit_history",
    Immutable:         true,
    Properties: [
        { Name: "previousContent", Type: string,   Required: true },
        { Name: "editedBy",        Type: string,   Required: true }, // participantID
        { Name: "version",         Type: integer,  Required: true }, // 1-based edit count
        { Name: "editedAt",        Type: datetime, Required: true },
    ],
}
```

One `EditHistory` document is created per save. The full edit trail is
accessible via `GET /{agencyId}/comm/messages/{messageId}/history` which
traverses the `has_edit` edges from the Message vertex.

---

### Attachment

```
TypeDefinition{
    Name:              "Attachment",
    DisplayName:       "Attachment",
    StorageCollection: "comm_attachments",
    Immutable:         true,
    Properties: [
        { Name: "fileName",  Type: string,  Required: true },
        { Name: "mimeType",  Type: string,  Required: true },
        { Name: "url",       Type: string,  Required: true },
        { Name: "sizeBytes", Type: integer, Required: false },
    ],
}
```

Attachments are immutable once uploaded. A `has_attachment` edge links the
Message vertex to its Attachment vertices.

---

## 3. Graph Relationship Model

All edges are stored in the `comm_relationships` **edge collection**.
The named graph `comm_graph` spans all Comm vertex collections.

| Relationship | From | To | Edge Properties | Semantics |
|---|---|---|---|---|
| `has_member` | Channel | Participant | `role` (admin/member), `joinedAt`, `invitedBy` | Participant is a member of a Channel |
| `has_message` | Channel | Message | `postedAt` | Message belongs to a Channel (or Thread root) |
| `is_reply_to` | Message | Message | — | Reply anchored on a thread-root or root message |
| `has_attachment` | Message | Attachment | `attachedAt` | File attached to a message |
| `has_reaction` | Message | Participant | `emoji`, `reactedAt` | Emoji reaction from a Participant on a Message |
| `read_by` | Message | Participant | `readAt` | Message read receipt for a Participant |
| `has_edit` | Message | EditHistory | `editedAt` | Ordered edit history trail |

### Graph Traversal Patterns

```
Channel membership:
  Channel --[has_member]--> Participant

Channel messages (flat, depth=1):
  Channel --[has_message]--> Message

Thread (all replies to a thread root, depth=n):
  ThreadRootMessage <--[is_reply_to]-- Message (inbound traversal)

Message reactions:
  Message --[has_reaction]--> Participant  (edge carries emoji + reactedAt)

Read receipts:
  Message --[read_by]--> Participant

Edit history:
  Message --[has_edit]--> EditHistory  (ordered by editedAt)

Attachments:
  Message --[has_attachment]--> Attachment
```

### Thread Model Detail

```
Channel
  │
  └──[has_message]──► Message A          ← normal message
  └──[has_message]──► Message B          ← promoted: isThreadRoot=true
                           ▲
                           │ is_reply_to
                      Message B1          ← thread reply
                           ▲
                           │ is_reply_to
                      Message B1a         ← reply to reply
```

From the channel view: Message B is visible with a thread activity preview
(reply count, last reply). Clicking zooms in to traverse
`Message B ←[is_reply_to]─ *` (inbound, any depth).

---

## 4. Pub/Sub Events

| Topic | Published when | Payload |
|---|---|---|
| `comm.message.sent` | Message created in a channel | `{messageID, channelID, authorID}` |
| `comm.message.edited` | Message content updated | `{messageID, channelID, version}` |
| `comm.thread.promoted` | Message promoted to thread root | `{messageID, channelID}` |
| `comm.reaction.added` | `has_reaction` edge created | `{messageID, participantID, emoji}` |
| `comm.member.joined` | `has_member` edge created | `{channelID, participantID, role}` |
| `comm.participant.presence` | Participant presence updated | `{participantID, presence}` |
