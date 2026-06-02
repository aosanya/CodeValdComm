package httphandler

import codevaldcomm "github.com/aosanya/CodeValdComm"

const (
	TopicMessageSent      = codevaldcomm.TopicMessageSent
	TopicMessageEdited    = codevaldcomm.TopicMessageEdited
	TopicMessageDelivered = codevaldcomm.TopicMessageDelivered
	TopicMessageFailed    = codevaldcomm.TopicMessageFailed
	TopicThreadPromoted   = codevaldcomm.TopicThreadPromoted
	TopicMemberJoined     = codevaldcomm.TopicMemberJoined
)

// Payload type aliases — defined in the root package.
type MessageSentPayload      = codevaldcomm.MessageSentPayload
type MessageEditedPayload    = codevaldcomm.MessageEditedPayload
type MessageDeliveredPayload = codevaldcomm.MessageDeliveredPayload
type MessageFailedPayload    = codevaldcomm.MessageFailedPayload

// AllTopics delegates to the root package.
func AllTopics() []string { return codevaldcomm.AllTopics() }
