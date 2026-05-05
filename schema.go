// Package codevaldcomm — pre-delivered schema definition.
//
// This file exposes [DefaultCommSchema], which returns the fixed [types.Schema]
// for CodeValdComm. cmd/main.go seeds this schema idempotently on startup via
// SchemaManager.SetSchema.
//
// The schema declares five TypeDefinitions:
//   - Channel      — a messaging channel (group or direct); mutable (StorageCollection: comm_groups)
//   - Participant  — a channel member / user presence record; mutable (comm_participants)
//   - Message      — a single message in a channel; mutable (comm_messages)
//   - EditHistory  — immutable snapshot of a previous message body (comm_edit_history)
//   - Attachment   — immutable file reference attached to a message (comm_attachments)
//
// And seven RelationshipDefinitions:
//
//	Channel ──has_member──► Participant   (ToMany: true)
//	Channel ──has_message──► Message      (ToMany: true)
//	Message ──is_reply_to──► Message      (ToMany: false)
//	Message ──has_attachment──► Attachment (ToMany: true)
//	Message ──has_reaction──► Participant  (ToMany: true)
//	Message ──read_by──► Participant       (ToMany: true)
//	Message ──has_edit──► EditHistory      (ToMany: true)
//
// Storage:
//   - Channel      → "comm_groups"       document collection
//   - Participant  → "comm_participants" document collection
//   - Message      → "comm_messages"     document collection
//   - EditHistory  → "comm_edit_history" document collection (immutable)
//   - Attachment   → "comm_attachments"  document collection (immutable)
//   - All edges    → "comm_relationships" ArangoDB edge collection
package codevaldcomm

import "github.com/aosanya/CodeValdSharedLib/types"

// DefaultCommSchema returns the pre-delivered [types.Schema] seeded by
// cmd/main.go on startup. The operation is idempotent — calling SetSchema
// multiple times with the same schema ID is safe.
func DefaultCommSchema() types.Schema {
	return types.Schema{
		ID:      "comm-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []types.TypeDefinition{
			{
				Name:              "Channel",
				DisplayName:       "Channel",
				PathSegment:       "channels",
				EntityIDParam:     "channelId",
				StorageCollection: "comm_groups",
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString},
					// is_direct distinguishes DM channels from group channels.
					{Name: "is_direct", Type: types.PropertyTypeBoolean},
					// edit_window_seconds: 0 = editing disabled; -1 = always editable;
					// >0 = seconds since message.created_at within which edits are allowed.
					{Name: "edit_window_seconds", Type: types.PropertyTypeInteger},
					{Name: "created_at", Type: types.PropertyTypeString},
					{Name: "updated_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "has_member",
						Label:       "Members",
						PathSegment: "members",
						ToType:      "Participant",
						ToMany:      true,
						Inverse:     "member_of",
					},
					{
						Name:        "has_message",
						Label:       "Messages",
						PathSegment: "messages",
						ToType:      "Message",
						ToMany:      true,
						Inverse:     "belongs_to_channel",
					},
				},
			},
			{
				Name:              "Participant",
				DisplayName:       "Participant",
				PathSegment:       "participants",
				EntityIDParam:     "participantId",
				StorageCollection: "comm_participants",
				Properties: []types.PropertyDefinition{
					{Name: "user_id", Type: types.PropertyTypeString, Required: true},
					{Name: "display_name", Type: types.PropertyTypeString},
					// presence: "online" | "away" | "offline"
					{Name: "presence", Type: types.PropertyTypeString},
					{Name: "last_seen_at", Type: types.PropertyTypeString},
					{Name: "created_at", Type: types.PropertyTypeString},
					{Name: "updated_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "member_of",
						Label:       "Channels",
						PathSegment: "channels",
						ToType:      "Channel",
						ToMany:      true,
						Inverse:     "has_member",
					},
				},
			},
			{
				Name:              "Message",
				DisplayName:       "Message",
				PathSegment:       "messages",
				EntityIDParam:     "messageId",
				StorageCollection: "comm_messages",
				Properties: []types.PropertyDefinition{
					{Name: "body", Type: types.PropertyTypeString, Required: true},
					{Name: "sender_id", Type: types.PropertyTypeString, Required: true},
					// is_thread_root: true when this message has been promoted to a thread head.
					{Name: "is_thread_root", Type: types.PropertyTypeBoolean},
					{Name: "created_at", Type: types.PropertyTypeString},
					{Name: "updated_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "belongs_to_channel",
						Label:       "Channel",
						PathSegment: "channel",
						ToType:      "Channel",
						ToMany:      false,
						Inverse:     "has_message",
					},
					{
						Name:        "is_reply_to",
						Label:       "Parent Message",
						PathSegment: "parent",
						ToType:      "Message",
						ToMany:      false,
					},
					{
						Name:        "has_attachment",
						Label:       "Attachments",
						PathSegment: "attachments",
						ToType:      "Attachment",
						ToMany:      true,
					},
					{
						Name:        "has_reaction",
						Label:       "Reactions",
						PathSegment: "reactions",
						ToType:      "Participant",
						ToMany:      true,
						Properties: []types.PropertyDefinition{
							// emoji is the reaction character/shortcode (e.g. "👍" or ":thumbsup:").
							{Name: "emoji", Type: types.PropertyTypeString, Required: true},
						},
					},
					{
						Name:        "read_by",
						Label:       "Read By",
						PathSegment: "read-by",
						ToType:      "Participant",
						ToMany:      true,
					},
					{
						Name:        "has_edit",
						Label:       "Edit History",
						PathSegment: "edits",
						ToType:      "EditHistory",
						ToMany:      true,
					},
				},
			},
			{
				Name:              "EditHistory",
				DisplayName:       "Edit History",
				PathSegment:       "edit-history",
				EntityIDParam:     "editHistoryId",
				StorageCollection: "comm_edit_history",
				// EditHistory records are immutable snapshots — never update them.
				Immutable: true,
				Properties: []types.PropertyDefinition{
					// previous_body is the message body before the edit.
					{Name: "previous_body", Type: types.PropertyTypeString, Required: true},
					// message_id is the entitygraph ID of the owning Message.
					{Name: "message_id", Type: types.PropertyTypeString, Required: true},
					{Name: "edited_at", Type: types.PropertyTypeString, Required: true},
					{Name: "editor_id", Type: types.PropertyTypeString},
				},
			},
			{
				Name:              "Attachment",
				DisplayName:       "Attachment",
				PathSegment:       "attachments",
				EntityIDParam:     "attachmentId",
				StorageCollection: "comm_attachments",
				// Attachments are immutable once uploaded.
				Immutable: true,
				Properties: []types.PropertyDefinition{
					{Name: "filename", Type: types.PropertyTypeString, Required: true},
					{Name: "mime_type", Type: types.PropertyTypeString},
					{Name: "size_bytes", Type: types.PropertyTypeInteger},
					// url is the storage URL of the uploaded file.
					{Name: "url", Type: types.PropertyTypeString, Required: true},
					{Name: "created_at", Type: types.PropertyTypeString},
				},
			},
		},
	}
}
