// workflow_run_rollback.go — CodeValdComm leg of the WorkflowRun rollback
// coordinator (FEAT-20260602-004 Phase 2 — Comm service).
//
// CodeValdWork's coordinator calls RollbackByWorkflowRun on every service in
// the affected closure. The Comm rule from the FEAT spec is:
//
//   - Find every channel that holds at least one Message tagged with the
//     rolled-back workflow_run_id.
//   - Post one "pipeline rolled back" follow-up Message into each such
//     channel as a synthetic notification. Prior Messages are preserved —
//     recipients may have already read them, and they form part of the
//     audit trail.
//   - Channels that already received a rollback notification for this run
//     (a Message with rollback_notification=true and matching workflow_run_id)
//     are skipped, so retry after a partial rollback_failed is observably
//     idempotent.
package codevaldcomm

import (
	"context"
	"fmt"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// RollbackSenderID is the synthetic sender_id stamped on rollback notification
// Messages. Frontends can render these distinctly (e.g. a system badge).
const RollbackSenderID = "codevald.rollback"

// RollbackByWorkflowRunResult summarises the per-channel outcomes of a
// RollbackByWorkflowRun call. NotifiedChannelIDs and NotificationMessageIDs
// are aligned 1:1 — index i in one refers to the same channel as index i in
// the other.
type RollbackByWorkflowRunResult struct {
	WorkflowRunID          string
	NotifiedChannelIDs     []string
	SkippedChannelIDs      []string
	NotificationMessageIDs []string
}

// RollbackByWorkflowRun applies the Comm leg of the rollback coordinator for
// the given agency and workflow run. It is safe to call repeatedly: channels
// already holding a rollback notification for this run end up in
// SkippedChannelIDs without a second notification being posted.
//
// pub may be nil — the comm.pipeline.rolled_back event is silently dropped
// when no publisher is configured (matches Handler.publish semantics).
func RollbackByWorkflowRun(
	ctx context.Context,
	dm CommDataManager,
	pub CrossPublisher,
	agencyID, workflowRunID, reason string,
) (RollbackByWorkflowRunResult, error) {
	if workflowRunID == "" {
		return RollbackByWorkflowRunResult{}, ErrWorkflowRunIDRequired
	}

	result := RollbackByWorkflowRunResult{WorkflowRunID: workflowRunID}

	messages, err := dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Message",
		Properties: map[string]any{
			"workflow_run_id": workflowRunID,
		},
	})
	if err != nil {
		return result, fmt.Errorf("RollbackByWorkflowRun %s: list messages: %w", workflowRunID, err)
	}

	// Partition the closure into channels that need a notification and
	// channels that already received one. We must visit messages in a
	// deterministic order so the result slices are stable for tests.
	type channelState struct {
		channelID            string
		alreadyNotified      bool
	}
	seen := make(map[string]int) // channelID -> index in order
	var order []channelState

	for _, msg := range messages {
		channelID, channelErr := resolveOwningChannel(ctx, dm, agencyID, msg.ID)
		if channelErr != nil {
			return result, fmt.Errorf("RollbackByWorkflowRun %s: resolve channel for message %s: %w",
				workflowRunID, msg.ID, channelErr)
		}
		if channelID == "" {
			// Orphan message — no owning channel. Nothing to notify into.
			continue
		}

		idx, exists := seen[channelID]
		if !exists {
			seen[channelID] = len(order)
			order = append(order, channelState{channelID: channelID})
			idx = len(order) - 1
		}

		if isRollbackNotification(msg) {
			order[idx].alreadyNotified = true
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, ch := range order {
		if ch.alreadyNotified {
			result.SkippedChannelIDs = append(result.SkippedChannelIDs, ch.channelID)
			continue
		}

		msgID, postErr := postRollbackNotification(ctx, dm, agencyID, ch.channelID, workflowRunID, reason, now)
		if postErr != nil {
			return result, fmt.Errorf("RollbackByWorkflowRun %s: notify channel %s: %w",
				workflowRunID, ch.channelID, postErr)
		}
		result.NotifiedChannelIDs = append(result.NotifiedChannelIDs, ch.channelID)
		result.NotificationMessageIDs = append(result.NotificationMessageIDs, msgID)

		publishPipelineRolledBack(ctx, pub, agencyID, PipelineRolledBackPayload{
			ChannelID:     ch.channelID,
			MessageID:     msgID,
			WorkflowRunID: workflowRunID,
			Reason:        reason,
		})
	}

	return result, nil
}

// resolveOwningChannel finds the channel that owns the given message via the
// has_message edge (Channel→Message). Returns an empty string when no owning
// channel exists (orphan message).
func resolveOwningChannel(ctx context.Context, dm CommDataManager, agencyID, messageID string) (string, error) {
	rels, err := dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: agencyID,
		Name:     "has_message",
		ToID:     messageID,
	})
	if err != nil {
		return "", err
	}
	if len(rels) == 0 {
		return "", nil
	}
	return rels[0].FromID, nil
}

// isRollbackNotification reports whether the given Message is a synthetic
// "pipeline rolled back" notification. The rollback_notification property is
// stored as a JSON bool, which entitygraph hands back as bool or float64
// depending on the storage backend — both are accepted.
func isRollbackNotification(msg entitygraph.Entity) bool {
	switch v := msg.Properties["rollback_notification"].(type) {
	case bool:
		return v
	case float64:
		return v != 0
	default:
		return false
	}
}

// postRollbackNotification creates the synthetic Message and the has_message
// edge linking it to the channel. Returns the new Message ID.
func postRollbackNotification(
	ctx context.Context,
	dm CommDataManager,
	agencyID, channelID, workflowRunID, reason, now string,
) (string, error) {
	body := rollbackNotificationBody(workflowRunID, reason)
	props := map[string]any{
		"body":                  body,
		"sender_id":             RollbackSenderID,
		"workflow_run_id":       workflowRunID,
		"rollback_notification": true,
		"created_at":            now,
		"updated_at":            now,
	}
	if reason != "" {
		props["rollback_reason"] = reason
	}

	msg, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     "Message",
		Properties: props,
	})
	if err != nil {
		return "", fmt.Errorf("create notification message: %w", err)
	}

	if _, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID,
		Name:     "has_message",
		FromID:   channelID,
		ToID:     msg.ID,
	}); err != nil {
		return "", fmt.Errorf("link notification to channel: %w", err)
	}
	return msg.ID, nil
}

func rollbackNotificationBody(workflowRunID, reason string) string {
	if reason == "" {
		return fmt.Sprintf("Pipeline %s was rolled back.", workflowRunID)
	}
	return fmt.Sprintf("Pipeline %s was rolled back: %s", workflowRunID, reason)
}

// publishPipelineRolledBack emits the comm.pipeline.rolled_back event for the
// notified channel. Publish failures are swallowed by [eventbus.SafePublish] —
// per the platform-wide event-publishing rule, a publish error must never
// fail a domain mutation.
func publishPipelineRolledBack(ctx context.Context, pub CrossPublisher, agencyID string, payload PipelineRolledBackPayload) {
	if pub == nil {
		return
	}
	eventbus.SafePublish(ctx, pub, eventbus.Event{
		Topic:    TopicPipelineRolledBack,
		AgencyID: agencyID,
		Payload:  payload,
	})
}
