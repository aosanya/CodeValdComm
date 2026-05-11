package codevaldcomm

// Event topic constants — the closed set CodeValdComm publishes.
const (
	// TopicMessageSent fires after SendMessage completes.
	// Payload: [MessageSentPayload].
	TopicMessageSent = "comm.message.sent"
)

// AllTopics is the closed list of topics this service publishes.
func AllTopics() []string {
	return []string{TopicMessageSent}
}

// MessageSentPayload is the [eventbus.Event.Payload] for [TopicMessageSent].
type MessageSentPayload struct {
	ChannelID string
	MessageID string
	SenderID  string
}
