package codevaldcomm

import "github.com/aosanya/CodeValdSharedLib/eventbus"

// Event topic constants — the closed set CodeValdComm publishes.
const (
	// TopicMessageSent fires after SendMessage completes.
	TopicMessageSent = eventbus.DomainComm + "message.sent"
	// TopicMessageEdited fires after EditMessage completes.
	TopicMessageEdited = eventbus.DomainComm + "message.edited"
	// TopicMessageDelivered fires when a message is confirmed delivered to a participant.
	TopicMessageDelivered = eventbus.DomainComm + "message.delivered"
	// TopicMessageFailed fires when a pipeline-triggered message cannot be delivered.
	TopicMessageFailed = eventbus.DomainComm + "message.failed"
	// TopicThreadPromoted fires after PromoteToThread completes.
	TopicThreadPromoted = eventbus.DomainComm + "thread.promoted"
	// TopicMemberJoined fires after JoinChannel completes.
	TopicMemberJoined = eventbus.DomainComm + "member.joined"
	// TopicPipelineRolledBack fires once per channel that received a
	// rollback notification Message during RollbackByWorkflowRun.
	TopicPipelineRolledBack = eventbus.DomainComm + "pipeline.rolled_back"
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
		TopicPipelineRolledBack,
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

// PipelineRolledBackPayload is the [eventbus.Event.Payload] for
// [TopicPipelineRolledBack]. Published once per channel notified during a
// RollbackByWorkflowRun call.
type PipelineRolledBackPayload struct {
	ChannelID     string
	MessageID     string
	WorkflowRunID string
	Reason        string
}
