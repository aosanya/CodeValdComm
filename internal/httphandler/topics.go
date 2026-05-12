package httphandler

import codevaldcomm "github.com/aosanya/CodeValdComm"

const (
	TopicMessageSent   = codevaldcomm.TopicMessageSent
	TopicMessageEdited = codevaldcomm.TopicMessageEdited
	TopicThreadPromoted = codevaldcomm.TopicThreadPromoted
	TopicMemberJoined  = codevaldcomm.TopicMemberJoined
)

// MessageSentPayload is the payload for [TopicMessageSent].
// Defined in the root package as [codevaldcomm.MessageSentPayload].
type MessageSentPayload = codevaldcomm.MessageSentPayload

// AllTopics delegates to the root package.
func AllTopics() []string { return codevaldcomm.AllTopics() }
