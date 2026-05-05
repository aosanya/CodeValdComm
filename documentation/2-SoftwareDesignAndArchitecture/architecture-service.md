# CodeValdComm — Architecture: Service

> Part of [architecture.md](architecture.md)

## 1. gRPC Service

CodeValdComm exposes a `CommService` gRPC interface. The gRPC handlers are
thin — they translate protobuf messages to domain models and delegate to
`CommDataManager` and `CommSchemaManager`.

```proto
service CommService {
    // Entity operations (delegates to CommDataManager)
    rpc CreateEntity(CreateEntityRequest)         returns (Entity);
    rpc GetEntity(GetEntityRequest)               returns (Entity);
    rpc UpdateEntity(UpdateEntityRequest)         returns (Entity);
    rpc DeleteEntity(DeleteEntityRequest)         returns (google.protobuf.Empty);
    rpc ListEntities(ListEntitiesRequest)         returns (ListEntitiesResponse);

    // Relationship operations (delegates to CommDataManager)
    rpc CreateRelationship(CreateRelationshipRequest) returns (Relationship);
    rpc GetRelationship(GetRelationshipRequest)       returns (Relationship);
    rpc DeleteRelationship(DeleteRelationshipRequest) returns (google.protobuf.Empty);
    rpc ListRelationships(ListRelationshipsRequest)   returns (ListRelationshipsResponse);
    rpc TraverseGraph(TraverseGraphRequest)           returns (TraverseGraphResponse);

    // Schema operations (delegates to CommSchemaManager)
    rpc GetSchema(GetSchemaRequest)               returns (Schema);
}
```

`SetSchema` is not exposed via gRPC — the schema is pre-delivered and seeded
internally. External callers cannot overwrite the Comm schema.

---

## 2. Convenience HTTP Routes

Convenience routes are thin HTTP handlers that map domain-semantic URLs to
the underlying entity-graph operations. All routes are registered with
CodeValdCross on startup so CodeValdHi can discover them via the service
registry.

All paths are prefixed with `/{agencyId}/comm`.

### Channels

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/channels` | `CreateEntity(typeID=Channel)` |
| `GET` | `/{agencyId}/comm/channels` | `ListEntities(typeID=Channel, isDirect=false)` |
| `GET` | `/{agencyId}/comm/channels/{channelId}` | `GetEntity` |
| `PUT` | `/{agencyId}/comm/channels/{channelId}` | `UpdateEntity` |
| `DELETE` | `/{agencyId}/comm/channels/{channelId}` | `DeleteEntity` |

### Channel Members

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/channels/{channelId}/members` | `CreateRelationship(has_member, Channel→Participant)` |
| `GET` | `/{agencyId}/comm/channels/{channelId}/members` | `TraverseGraph(channelId, has_member, outbound, depth=1)` |
| `DELETE` | `/{agencyId}/comm/channels/{channelId}/members/{participantId}` | `DeleteRelationship(has_member)` |

### Messages

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/channels/{channelId}/messages` | `CreateEntity(Message)` + `CreateRelationship(has_message, Channel→Message)` |
| `GET` | `/{agencyId}/comm/channels/{channelId}/messages` | `TraverseGraph(channelId, has_message, outbound, depth=1)` |
| `PUT` | `/{agencyId}/comm/messages/{messageId}` | See [EditMessage flow](architecture-flows.md#editMessage) |
| `DELETE` | `/{agencyId}/comm/messages/{messageId}` | `DeleteEntity(Message)` |

### Threads

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `PUT` | `/{agencyId}/comm/messages/{messageId}/promote` | `UpdateEntity(Message, isThreadRoot=true)` + Publish |
| `POST` | `/{agencyId}/comm/messages/{messageId}/replies` | `CreateEntity(Message)` + `CreateRelationship(is_reply_to, Reply→Root)` |
| `GET` | `/{agencyId}/comm/messages/{messageId}/thread` | `TraverseGraph(messageId, is_reply_to, inbound, depth=n)` |

### Edit History

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `GET` | `/{agencyId}/comm/messages/{messageId}/history` | `TraverseGraph(messageId, has_edit, outbound)` |

### Reactions

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/messages/{messageId}/reactions` | `CreateRelationship(has_reaction, Message→Participant, {emoji})` |
| `GET` | `/{agencyId}/comm/messages/{messageId}/reactions` | `TraverseGraph(messageId, has_reaction, outbound, depth=1)` |
| `DELETE` | `/{agencyId}/comm/messages/{messageId}/reactions/{reactionId}` | `DeleteRelationship(has_reaction)` |

### Read Receipts

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/messages/{messageId}/read` | `CreateRelationship(read_by, Message→Participant, {readAt})` |
| `GET` | `/{agencyId}/comm/messages/{messageId}/read` | `TraverseGraph(messageId, read_by, outbound, depth=1)` |

### Attachments

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/messages/{messageId}/attachments` | `CreateEntity(Attachment)` + `CreateRelationship(has_attachment, Message→Attachment)` |
| `GET` | `/{agencyId}/comm/messages/{messageId}/attachments` | `TraverseGraph(messageId, has_attachment, outbound, depth=1)` |

### Participants

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/participants` | `CreateEntity(Participant)` |
| `GET` | `/{agencyId}/comm/participants/{participantId}` | `GetEntity` |
| `PUT` | `/{agencyId}/comm/participants/{participantId}` | `UpdateEntity` (presence, displayName) |
| `DELETE` | `/{agencyId}/comm/participants/{participantId}` | `DeleteEntity` |

### Direct Messages

| Method | Path | Entity-Graph Operation |
|---|---|---|
| `POST` | `/{agencyId}/comm/direct` | `CreateEntity(Channel, isDirect=true)` + `CreateRelationship(has_member)` × 2 |
| `GET` | `/{agencyId}/comm/direct` | `ListEntities(Channel, isDirect=true, filter: caller is member)` |
| `GET` | `/{agencyId}/comm/direct/{channelId}/messages` | `TraverseGraph(channelId, has_message, outbound, depth=1)` |

---

## 3. Error Mapping

| Error | gRPC Status |
|---|---|
| `ErrEntityNotFound` | `codes.NotFound` |
| `ErrRelationshipNotFound` | `codes.NotFound` |
| `ErrSchemaNotFound` | `codes.NotFound` |
| `ErrInvalidEntity` | `codes.InvalidArgument` |
| `ErrInvalidRelationship` | `codes.InvalidArgument` |
| `ErrImmutableType` | `codes.FailedPrecondition` |
| unknown | `codes.Internal` |

---

## 4. CodeValdCross Registration

CodeValdComm registers with CodeValdCross on startup and sends a heartbeat
every 20 seconds.

```go
RegisterRequest{
    ServiceName: "codevaldcomm",
    Addr:        ":50060",
    Produces: []string{
        "cross.comm.{agencyID}.message.sent",
        "cross.comm.{agencyID}.message.edited",
        "cross.comm.{agencyID}.thread.promoted",
        "cross.comm.{agencyID}.reaction.added",
        "cross.comm.{agencyID}.member.joined",
        "cross.comm.{agencyID}.participant.presence",
    },
    Consumes: []string{
        "cross.agency.created",  // seed default schema for new agency
    },
    Routes: commRoutes(),  // all convenience routes listed above
}
```

When `cross.agency.created` is received, CodeValdComm calls
`CommSchemaManager.SetSchema(agencyID, defaultCommSchema)` if no schema
exists for that agency yet.

---

## 5. Project Layout

```
CodeValdComm/
├── cmd/
│   └── main.go                    # Dependency wiring only
├── errors.go                      # ErrEntityNotFound, ErrImmutableType, etc.
├── models.go                      # CommDataManager, CommSchemaManager type aliases
├── schema.go                      # defaultCommSchema — pre-delivered TypeDefinitions
├── go.mod
├── internal/
│   ├── manager/
│   │   └── manager.go             # Concrete CommDataManager implementation
│   ├── server/
│   │   └── server.go              # gRPC CommService handlers
│   ├── httphandler/
│   │   └── handler.go             # Convenience HTTP route handlers
│   ├── config/
│   │   └── config.go              # Config struct + loader
│   └── registrar/
│       └── registrar.go           # CodeValdCross heartbeat + schema seeding
├── storage/
│   └── arangodb/
│       └── backend.go             # ArangoDB Backend implementation
└── proto/
    └── codevaldcomm/
        └── v1/
            └── comm.proto
```
