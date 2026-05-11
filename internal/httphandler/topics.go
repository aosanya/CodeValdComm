package httphandler

import codevaldcomm "github.com/aosanya/CodeValdComm"

// TopicMessageSent is the only event this service publishes.
// Defined in the root package as [codevaldcomm.TopicMessageSent].
const TopicMessageSent = codevaldcomm.TopicMessageSent

// MessageSentPayload is the payload for [TopicMessageSent].
// Defined in the root package as [codevaldcomm.MessageSentPayload].
type MessageSentPayload = codevaldcomm.MessageSentPayload

// AllTopics delegates to the root package.
func AllTopics() []string { return codevaldcomm.AllTopics() }
