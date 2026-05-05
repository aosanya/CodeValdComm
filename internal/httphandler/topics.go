package httphandler

// Comm event topic constants — the closed set CodeValdComm publishes.
// Topic format: cross.comm.{agencyID}.{event}
const (
	// TopicMessageSent fires after SendMessage completes. Payload: MessageSentPayload.
	TopicMessageSent = "cross.comm.message.sent"

	// TopicMessageEdited fires after EditMessage completes. Payload: MessageEditedPayload.
	TopicMessageEdited = "cross.comm.message.edited"

	// TopicThreadPromoted fires after PromoteToThread completes. Payload: ThreadPromotedPayload.
	TopicThreadPromoted = "cross.comm.thread.promoted"

	// TopicReactionAdded fires after AddReaction completes. Payload: ReactionAddedPayload.
	TopicReactionAdded = "cross.comm.reaction.added"

	// TopicMemberJoined fires after JoinChannel completes. Payload: MemberJoinedPayload.
	TopicMemberJoined = "cross.comm.member.joined"

	// TopicParticipantPresence fires after UpdatePresence completes. Payload: ParticipantPresencePayload.
	TopicParticipantPresence = "cross.comm.participant.presence"
)

// AllTopics returns the closed list of topics this service publishes.
func AllTopics() []string {
	return []string{
		TopicMessageSent,
		TopicMessageEdited,
		TopicThreadPromoted,
		TopicReactionAdded,
		TopicMemberJoined,
		TopicParticipantPresence,
	}
}

// ── Payload types ─────────────────────────────────────────────────────────────

// MessageSentPayload is the eventbus.Event.Payload for TopicMessageSent.
type MessageSentPayload struct {
	ChannelID string
	MessageID string
	SenderID  string
}

// MessageEditedPayload is the eventbus.Event.Payload for TopicMessageEdited.
type MessageEditedPayload struct {
	MessageID   string
	EditHistoryID string
}

// ThreadPromotedPayload is the eventbus.Event.Payload for TopicThreadPromoted.
type ThreadPromotedPayload struct {
	MessageID string
}

// ReactionAddedPayload is the eventbus.Event.Payload for TopicReactionAdded.
type ReactionAddedPayload struct {
	MessageID     string
	ParticipantID string
	Emoji         string
}

// MemberJoinedPayload is the eventbus.Event.Payload for TopicMemberJoined.
type MemberJoinedPayload struct {
	ChannelID     string
	ParticipantID string
}

// ParticipantPresencePayload is the eventbus.Event.Payload for TopicParticipantPresence.
type ParticipantPresencePayload struct {
	ParticipantID string
	Presence      string
}
