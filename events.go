package codevaldcomm

// Event topic constants — the closed set CodeValdComm publishes.
const (
	// TopicMessageSent fires after SendMessage completes.
	TopicMessageSent = "comm.message.sent"
	// TopicMessageEdited fires after EditMessage completes.
	TopicMessageEdited = "comm.message.edited"
	// TopicThreadPromoted fires after PromoteToThread completes.
	TopicThreadPromoted = "comm.thread.promoted"
	// TopicMemberJoined fires after JoinChannel completes.
	TopicMemberJoined = "comm.member.joined"
)

// AllTopics is the closed list of topics this service publishes.
func AllTopics() []string {
	return []string{TopicMessageSent, TopicMessageEdited, TopicThreadPromoted, TopicMemberJoined}
}

// MessageSentPayload is the [eventbus.Event.Payload] for [TopicMessageSent].
type MessageSentPayload struct {
	ChannelID string
	MessageID string
	SenderID  string
}
