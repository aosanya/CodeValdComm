# CodeValdComm — Documentation

## Index

| Section | Contents |
|---|---|
| [1 — Software Requirements](1-SoftwareRequirements/requirements.md) | Functional requirements, NFR, stakeholders |
| [2 — Software Design & Architecture](2-SoftwareDesignAndArchitecture/architecture.md) | Architecture index |
| [3 — Software Development](3-SofwareDevelopment/mvp.md) | MVP task list and status |
| [4 — QA](4-QA/README.md) | Testing strategy |

## What is CodeValdComm?

CodeValdComm is a **Go gRPC microservice** that provides the communication layer
of the CodeVald platform. It manages channels (group chats and direct messages),
messages, threaded conversations, reactions, read receipts, file attachments,
participants, and presence.

Unlike CodeValdDT — where agencies define their own entity types via a custom
schema — CodeValdComm ships with a **pre-delivered, fixed schema**. The
TypeDefinitions for Channel, Message, Participant, Attachment, and EditHistory
are baked in. Agencies do not configure them; they use them.

All data is modelled as **entities and graph relationships** using the shared
`entitygraph.DataManager` / `entitygraph.SchemaManager` interfaces from
`CodeValdSharedLib`. Convenience HTTP routes expose domain-semantic endpoints
(e.g. `GET /{agencyId}/comm/channels/{channelId}/messages`) that map
transparently onto the underlying entity-graph operations.
