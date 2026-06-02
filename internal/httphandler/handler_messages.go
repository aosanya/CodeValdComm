package httphandler

import (
	"errors"
	"net/http"
	"time"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── (1) SendMessage ───────────────────────────────────────────────────────────

type sendMessageRequest struct {
	Body          string `json:"body"`
	SenderID      string `json:"sender_id"`
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request, channelID string) {
	ctx := r.Context()
	aid := agencyID(r)

	var req sendMessageRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Body == "" || req.SenderID == "" {
		writeError(w, http.StatusBadRequest, "body and sender_id are required")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	props := map[string]any{
		"body":       req.Body,
		"sender_id":  req.SenderID,
		"created_at": now,
		"updated_at": now,
	}
	if req.WorkflowRunID != "" {
		props["workflow_run_id"] = req.WorkflowRunID
	}

	msg, err := h.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   aid,
		TypeID:     "Message",
		Properties: props,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: aid,
		Name:     "has_message",
		FromID:   channelID,
		ToID:     msg.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicMessageSent, MessageSentPayload{
		ChannelID:     channelID,
		MessageID:     msg.ID,
		SenderID:      req.SenderID,
		WorkflowRunID: req.WorkflowRunID,
	})
	writeJSON(w, http.StatusCreated, msg)
}

// ── (2) PromoteToThread ───────────────────────────────────────────────────────

func (h *Handler) promoteToThread(w http.ResponseWriter, r *http.Request, messageID string) {
	ctx := r.Context()
	aid := agencyID(r)

	msg, err := h.dm.GetEntity(ctx, aid, messageID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Idempotent: skip update if already a thread root.
	if isThreadRoot, _ := msg.Properties["is_thread_root"].(bool); isThreadRoot {
		writeJSON(w, http.StatusOK, msg)
		return
	}

	updated, err := h.dm.UpdateEntity(ctx, aid, messageID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"is_thread_root": true,
			"updated_at":     time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicThreadPromoted, map[string]string{"message_id": messageID})
	writeJSON(w, http.StatusOK, updated)
}

// ── (3) EditMessage ───────────────────────────────────────────────────────────

type editMessageRequest struct {
	Body     string `json:"body"`
	EditorID string `json:"editor_id"`
}

func (h *Handler) editMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	ctx := r.Context()
	aid := agencyID(r)

	var req editMessageRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}

	msg, err := h.dm.GetEntity(ctx, aid, messageID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Resolve owning channel to check editWindowSeconds.
	channels, err := h.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: aid,
		FromID:   messageID,
		Name:     "belongs_to_channel",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(channels) > 0 {
		ch, chErr := h.dm.GetEntity(ctx, aid, channels[0].ToID)
		if chErr == nil {
			if closed, closeErr := h.checkEditWindow(ch, msg); closed {
				if closeErr != nil {
					writeError(w, http.StatusForbidden, closeErr.Error())
				} else {
					writeError(w, http.StatusForbidden, codevaldcomm.ErrEditWindowClosed.Error())
				}
				return
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	previousBody, _ := msg.Properties["body"].(string)
	workflowRunID, _ := msg.Properties["workflow_run_id"].(string)

	// Create EditHistory snapshot.
	eh, err := h.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: aid,
		TypeID:   "EditHistory",
		Properties: map[string]any{
			"previous_body": previousBody,
			"message_id":    messageID,
			"edited_at":     now,
			"editor_id":     req.EditorID,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Link Message → EditHistory.
	if _, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: aid,
		Name:     "has_edit",
		FromID:   messageID,
		ToID:     eh.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	editCount, _ := msg.Properties["edit_count"].(float64)
	updated, err := h.dm.UpdateEntity(ctx, aid, messageID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"body":       req.Body,
			"updated_at": now,
			"edited":     true,
			"edit_count": int(editCount) + 1,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicMessageEdited, MessageEditedPayload{
		MessageID:     messageID,
		WorkflowRunID: workflowRunID,
	})
	writeJSON(w, http.StatusOK, updated)
}

// checkEditWindow returns (true, err) when editing is not allowed.
// editWindowSeconds: 0 = disabled; -1 = always allowed; >0 = seconds from createdAt.
func (h *Handler) checkEditWindow(channel, msg entitygraph.Entity) (closed bool, err error) {
	var windowSecs int64
	switch v := channel.Properties["edit_window_seconds"].(type) {
	case float64:
		windowSecs = int64(v)
	case int64:
		windowSecs = v
	case int:
		windowSecs = int64(v)
	default:
		return false, nil
	}

	if windowSecs == 0 {
		return true, codevaldcomm.ErrEditWindowClosed
	}
	if windowSecs < 0 {
		return false, nil
	}

	createdAtStr, _ := msg.Properties["created_at"].(string)
	if createdAtStr == "" {
		return false, nil
	}
	createdAt, parseErr := time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return false, nil
	}
	if time.Since(createdAt) > time.Duration(windowSecs)*time.Second {
		return true, codevaldcomm.ErrEditWindowClosed
	}
	return false, nil
}

// ── (11) ListMessages ─────────────────────────────────────────────────────────

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request, channelID string) {
	ctx := r.Context()
	aid := agencyID(r)

	rels, err := h.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: aid,
		FromID:   channelID,
		Name:     "has_message",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	messages := make([]entitygraph.Entity, 0, len(rels))
	for _, rel := range rels {
		msg, err := h.dm.GetEntity(ctx, aid, rel.ToID)
		if err != nil {
			continue
		}
		// Apply optional workflow_run_id filter.
		if wrid := r.URL.Query().Get("workflow_run_id"); wrid != "" {
			if v, _ := msg.Properties["workflow_run_id"].(string); v != wrid {
				continue
			}
		}
		messages = append(messages, msg)
	}
	writeJSON(w, http.StatusOK, messages)
}

// ── (12) ListMessagesByWorkflowRun ────────────────────────────────────────────

func (h *Handler) listMessagesByWorkflowRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	aid := agencyID(r)

	wrid := r.URL.Query().Get("workflow_run_id")
	if wrid == "" {
		writeError(w, http.StatusBadRequest, "workflow_run_id query parameter is required")
		return
	}

	messages, err := h.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: aid,
		TypeID:   "Message",
		Properties: map[string]any{
			"workflow_run_id": wrid,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, messages)
}
