package codevaldcomm

// Event topic constants — the closed set CodeValdComm publishes.
const (
	// TopicMessageSent fires after SendMessage completes.
	TopicMessageSent = "comm.message.sent"
	// TopicMessageEdited fires after EditMessage completes.
	TopicMessageEdited = "comm.message.edited"
	// TopicMessageDelivered fires when a message is confirmed delivered to a participant.
	TopicMessageDelivered = "comm.message.delivered"
	// TopicMessageFailed fires when a pipeline-triggered message cannot be delivered.
	TopicMessageFailed = "comm.message.failed"
	// TopicThreadPromoted fires after PromoteToThread completes.
	TopicThreadPromoted = "comm.thread.promoted"
	// TopicMemberJoined fires after JoinChannel completes.
	TopicMemberJoined = "comm.member.joined"
)

// AllTopics is the closed list of topics this service publishes.
func AllTopics() []string {
	return []string{
		TopicMessageSent,
		TopicMessageEdited,
		TopicMessageDelivered,
		TopicMessageFailed,
		TopicThreadPromoted,
		TopicMemberJoined,
	}
}

// MessageSentPayload is the [eventbus.Event.Payload] for [TopicMessageSent].
type MessageSentPayload struct {
	ChannelID     string
	MessageID     string
	SenderID      string
	WorkflowRunID string
}

// MessageEditedPayload is the [eventbus.Event.Payload] for [TopicMessageEdited].
type MessageEditedPayload struct {
	MessageID     string
	WorkflowRunID string
}

// MessageDeliveredPayload is the [eventbus.Event.Payload] for [TopicMessageDelivered].
type MessageDeliveredPayload struct {
	MessageID     string
	ParticipantID string
	WorkflowRunID string
}

// MessageFailedPayload is the [eventbus.Event.Payload] for [TopicMessageFailed].
type MessageFailedPayload struct {
	MessageID     string
	Reason        string
	WorkflowRunID string
}
