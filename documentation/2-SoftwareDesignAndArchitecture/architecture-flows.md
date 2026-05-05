# CodeValdComm — Architecture: Flows

> Part of [architecture.md](architecture.md)

## Error Types

| Error | gRPC Code | When |
|---|---|---|
| `ErrEntityNotFound` | `codes.NotFound` | Entity does not exist |
| `ErrRelationshipNotFound` | `codes.NotFound` | Edge does not exist |
| `ErrSchemaNotFound` | `codes.NotFound` | Agency schema not seeded |
| `ErrInvalidEntity` | `codes.InvalidArgument` | Required property missing or invalid |
| `ErrInvalidRelationship` | `codes.InvalidArgument` | Invalid edge endpoints |
| `ErrImmutableType` | `codes.FailedPrecondition` | UpdateEntity called on immutable type |
| `ErrEditWindowClosed` | `codes.FailedPrecondition` | Message edit attempted after window expired |

---

## Flow 1: SendMessage (POST /{agencyId}/comm/channels/{channelId}/messages)

**Inputs:** `agencyID`, `channelID`, `authorID`, `content`

```
1. Validate inputs — content must not be empty; authorID must be non-empty
2. commDataManager.GetEntity(ctx, agencyID, channelID)
   → ErrEntityNotFound if channel does not exist
3. commDataManager.CreateEntity(ctx, CreateEntityRequest{
       AgencyID: agencyID,
       TypeID:   "Message",
       Properties: { content, authorID, isThreadRoot: false, editCount: 0 },
   })
   → ErrInvalidEntity on validation failure
   → returns message entity with generated ID
4. commDataManager.CreateRelationship(ctx, CreateRelationshipRequest{
       AgencyID:  agencyID,
       Label:     "has_message",
       FromID:    channelID,
       FromType:  "comm_groups",
       ToID:      message.ID,
       ToType:    "comm_messages",
       Properties: { postedAt: time.Now().UTC() },
   })
5. bus.Publish(ctx, Message{
       ID:      uuid.New().String(),
       Topic:   "cross.comm." + agencyID + ".message.sent",
       Payload: { messageID: message.ID, channelID, authorID },
       Source:  "codevaldcomm",
   })
6. Return message entity
```

---

## Flow 2: PromoteToThread (PUT /{agencyId}/comm/messages/{messageId}/promote)

**Inputs:** `agencyID`, `messageID`

```
1. Validate inputs
2. commDataManager.GetEntity(ctx, agencyID, messageID)
   → ErrEntityNotFound if message does not exist
   → early return if message.isThreadRoot == true (idempotent)
3. commDataManager.UpdateEntity(ctx, UpdateEntityRequest{
       AgencyID: agencyID,
       EntityID: messageID,
       Properties: { isThreadRoot: true },
   })
4. bus.Publish(ctx, Message{
       ID:      uuid.New().String(),
       Topic:   "cross.comm." + agencyID + ".thread.promoted",
       Payload: { messageID, channelID: resolveChannelID(ctx, messageID) },
       Source:  "codevaldcomm",
   })
5. Return updated message entity
```

`resolveChannelID` traverses `has_message` inbound to find the parent Channel.

---

## Flow 3: EditMessage (PUT /{agencyId}/comm/messages/{messageId})

**Inputs:** `agencyID`, `messageID`, `editorID`, `newContent`

```
1. Validate inputs — newContent must not be empty
2. commDataManager.GetEntity(ctx, agencyID, messageID)
   → ErrEntityNotFound if message does not exist
3. Resolve parent channel via inbound has_message edge
4. commDataManager.GetEntity(ctx, agencyID, channelID)
   → read editWindowSeconds from channel
5. Check edit window:
   - if editWindowSeconds == 0:
       return ErrEditWindowClosed
   - if editWindowSeconds > 0:
       elapsed = time.Since(message.createdAt)
       if elapsed > editWindowSeconds:
           return ErrEditWindowClosed
   - if editWindowSeconds == -1:
       allow (always editable)
6. commDataManager.CreateEntity(ctx, CreateEntityRequest{
       AgencyID: agencyID,
       TypeID:   "EditHistory",
       Properties: {
           previousContent: message.content,
           editedBy:        editorID,
           version:         message.editCount + 1,
           editedAt:        time.Now().UTC(),
       },
   })
   → returns editHistory entity
7. commDataManager.CreateRelationship(ctx, CreateRelationshipRequest{
       AgencyID:   agencyID,
       Label:      "has_edit",
       FromID:     messageID,
       FromType:   "comm_messages",
       ToID:       editHistory.ID,
       ToType:     "comm_edit_history",
       Properties: { editedAt: editHistory.editedAt },
   })
8. commDataManager.UpdateEntity(ctx, UpdateEntityRequest{
       AgencyID: agencyID,
       EntityID: messageID,
       Properties: {
           content:   newContent,
           editCount: message.editCount + 1,
           editedAt:  time.Now().UTC(),
       },
   })
9. bus.Publish(ctx, Message{
       ID:      uuid.New().String(),
       Topic:   "cross.comm." + agencyID + ".message.edited",
       Payload: { messageID, channelID, version: message.editCount + 1 },
       Source:  "codevaldcomm",
   })
10. Return updated message entity
```

---

## Flow 4: AddReaction (POST /{agencyId}/comm/messages/{messageId}/reactions)

**Inputs:** `agencyID`, `messageID`, `participantID`, `emoji`

```
1. Validate inputs — emoji must not be empty
2. commDataManager.GetEntity(ctx, agencyID, messageID)
   → ErrEntityNotFound if message does not exist
3. commDataManager.GetEntity(ctx, agencyID, participantID)
   → ErrEntityNotFound if participant does not exist
4. commDataManager.CreateRelationship(ctx, CreateRelationshipRequest{
       AgencyID:  agencyID,
       Label:     "has_reaction",
       FromID:    messageID,
       FromType:  "comm_messages",
       ToID:      participantID,
       ToType:    "comm_participants",
       Properties: { emoji, reactedAt: time.Now().UTC() },
   })
5. bus.Publish(ctx, Message{
       ID:      uuid.New().String(),
       Topic:   "cross.comm." + agencyID + ".reaction.added",
       Payload: { messageID, participantID, emoji },
       Source:  "codevaldcomm",
   })
6. Return relationship (edge) document
```

---

## Flow 5: MarkRead (POST /{agencyId}/comm/messages/{messageId}/read)

**Inputs:** `agencyID`, `messageID`, `participantID`

```
1. Validate inputs
2. commDataManager.GetEntity(ctx, agencyID, messageID)
   → ErrEntityNotFound if message does not exist
3. Check if read_by edge already exists for this (messageID, participantID) pair
   - if exists: return idempotently (no-op)
4. commDataManager.CreateRelationship(ctx, CreateRelationshipRequest{
       AgencyID:  agencyID,
       Label:     "read_by",
       FromID:    messageID,
       FromType:  "comm_messages",
       ToID:      participantID,
       ToType:    "comm_participants",
       Properties: { readAt: time.Now().UTC() },
   })
5. Return relationship document
```

No pub/sub event for MarkRead in v1 — read receipts are query-only.

---

## Flow 6: UpdatePresence (PUT /{agencyId}/comm/participants/{participantId})

**Inputs:** `agencyID`, `participantID`, `presence` (`online` | `away` | `offline`)

```
1. Validate inputs — presence must be one of the allowed values
2. commDataManager.GetEntity(ctx, agencyID, participantID)
   → ErrEntityNotFound if participant does not exist
3. commDataManager.UpdateEntity(ctx, UpdateEntityRequest{
       AgencyID:   agencyID,
       EntityID:   participantID,
       Properties: { presence },
   })
4. bus.Publish(ctx, Message{
       ID:      uuid.New().String(),
       Topic:   "cross.comm." + agencyID + ".participant.presence",
       Payload: { participantID, presence },
       Source:  "codevaldcomm",
   })
5. Return updated participant entity
```

---

## Flow 7: SchemaSeeding (on cross.agency.created)

**Trigger:** `cross.agency.created` pub/sub event from CodeValdCross

```
1. Extract agencyID from event payload
2. commSchemaManager.GetSchema(ctx, agencyID)
   - if schema exists: return (idempotent — do nothing)
   - if ErrSchemaNotFound: proceed to step 3
3. commSchemaManager.SetSchema(ctx, agencyID, defaultCommSchema)
   - defaultCommSchema is the package-level constant in schema.go
   - Contains TypeDefinitions for Channel, Participant, Message, EditHistory, Attachment
4. Log "codevaldcomm: seeded default schema for agency %s"
```

Schema seeding is the only write path to `comm_schemas`.
`SetSchema` is not exposed as a gRPC or HTTP endpoint.

---

## Flow 8: CreateDMChannel (POST /{agencyId}/comm/direct)

**Inputs:** `agencyID`, `fromParticipantID`, `toParticipantID`

```
1. Validate inputs — fromParticipantID != toParticipantID
2. Check if a DM channel already exists between the two participants
   (TraverseGraph both participants' has_member edges, intersect channels where isDirect=true)
   - if found: return existing channel (idempotent)
3. commDataManager.CreateEntity(ctx, CreateEntityRequest{
       AgencyID: agencyID,
       TypeID:   "Channel",
       Properties: {
           isDirect:  true,
           createdBy: fromParticipantID,
       },
   })
4. commDataManager.CreateRelationship(has_member, dmChannel → fromParticipant, role=member)
5. commDataManager.CreateRelationship(has_member, dmChannel → toParticipant,   role=member)
6. Return channel entity
```

---

## Flow 9: JoinChannel (POST /{agencyId}/comm/channels/{channelId}/members)

**Inputs:** `agencyID`, `channelID`, `participantID`, `role`, `invitedBy`

```
1. Validate inputs
2. commDataManager.GetEntity(ctx, agencyID, channelID)
   → ErrEntityNotFound if channel does not exist
3. commDataManager.GetEntity(ctx, agencyID, participantID)
   → ErrEntityNotFound if participant does not exist
4. Check if has_member edge already exists — idempotent if same role
5. commDataManager.CreateRelationship(ctx, CreateRelationshipRequest{
       AgencyID:   agencyID,
       Label:      "has_member",
       FromID:     channelID,
       FromType:   "comm_groups",
       ToID:       participantID,
       ToType:     "comm_participants",
       Properties: { role, joinedAt: time.Now().UTC(), invitedBy },
   })
6. bus.Publish(ctx, Message{
       ID:      uuid.New().String(),
       Topic:   "cross.comm." + agencyID + ".member.joined",
       Payload: { channelID, participantID, role },
       Source:  "codevaldcomm",
   })
7. Return relationship document
```
