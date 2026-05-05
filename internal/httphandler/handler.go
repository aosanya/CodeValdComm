// Package httphandler implements HTTP convenience routes for CodeValdComm.
// These routes orchestrate multi-step entity graph operations that cannot be
// expressed as a single EntityService CRUD call.
//
// The nine flows:
//  1. SendMessage      POST /channels/{channelId}/messages
//  2. PromoteToThread  PUT  /messages/{messageId}/promote
//  3. EditMessage      PUT  /messages/{messageId}
//  4. AddReaction      POST /messages/{messageId}/reactions
//  5. MarkRead         POST /messages/{messageId}/read
//  6. UpdatePresence   PUT  /participants/{participantId}
//  7. CreateChannel    POST /channels
//  8. JoinChannel      POST /channels/{channelId}/members
//  9. CreateDM         POST /direct
package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// Handler implements http.Handler for the nine comm convenience flows.
// Construct via New; serve alongside the gRPC server via cmux.
type Handler struct {
	dm  codevaldcomm.CommDataManager
	sm  codevaldcomm.CommSchemaManager
	pub codevaldcomm.CrossPublisher
}

// New constructs a Handler with the given managers and publisher.
// pub may be nil — events are silently dropped when no publisher is configured.
func New(dm codevaldcomm.CommDataManager, sm codevaldcomm.CommSchemaManager, pub codevaldcomm.CrossPublisher) *Handler {
	return &Handler{dm: dm, sm: sm, pub: pub}
}

// ServeHTTP routes incoming requests to the appropriate flow handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 4)

	switch {
	case r.Method == http.MethodPost && len(parts) >= 1 && parts[0] == "channels" && len(parts) == 1:
		h.createChannel(w, r)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "channels" && parts[2] == "messages":
		h.sendMessage(w, r, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "channels" && parts[2] == "members":
		h.joinChannel(w, r, parts[1])
	case r.Method == http.MethodPut && len(parts) == 3 && parts[0] == "messages" && parts[2] == "promote":
		h.promoteToThread(w, r, parts[1])
	case r.Method == http.MethodPut && len(parts) == 2 && parts[0] == "messages":
		h.editMessage(w, r, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "messages" && parts[2] == "reactions":
		h.addReaction(w, r, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "messages" && parts[2] == "read":
		h.markRead(w, r, parts[1])
	case r.Method == http.MethodPut && len(parts) == 2 && parts[0] == "participants":
		h.updatePresence(w, r, parts[1])
	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "direct":
		h.createDM(w, r)
	default:
		http.NotFound(w, r)
	}
}

// ── Helper utilities ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func agencyID(r *http.Request) string {
	return r.Header.Get("X-Agency-Id")
}

func (h *Handler) publish(ctx context.Context, agencyId, topic string, payload any) {
	if h.pub == nil {
		return
	}
	eventbus.SafePublish(ctx, h.pub, eventbus.Event{
		Topic:    topic,
		AgencyID: agencyId,
		Payload:  payload,
	})
}

// ── (1) SendMessage ───────────────────────────────────────────────────────────

type sendMessageRequest struct {
	Body     string `json:"body"`
	SenderID string `json:"sender_id"`
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
	msg, err := h.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: aid,
		TypeID:   "Message",
		Properties: map[string]any{
			"body":       req.Body,
			"sender_id":  req.SenderID,
			"created_at": now,
			"updated_at": now,
		},
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
		ChannelID: channelID,
		MessageID: msg.ID,
		SenderID:  req.SenderID,
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

	h.publish(ctx, aid, TopicThreadPromoted, ThreadPromotedPayload{MessageID: messageID})
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

	// Fetch the message.
	msg, err := h.dm.GetEntity(ctx, aid, messageID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Resolve the owning channel to check editWindowSeconds.
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

	// Update the message body.
	updated, err := h.dm.UpdateEntity(ctx, aid, messageID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"body":       req.Body,
			"updated_at": now,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicMessageEdited, MessageEditedPayload{
		MessageID:     messageID,
		EditHistoryID: eh.ID,
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
		// No window configured — default to always editable.
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

// ── (4) AddReaction ───────────────────────────────────────────────────────────

type addReactionRequest struct {
	ParticipantID string `json:"participant_id"`
	Emoji         string `json:"emoji"`
}

func (h *Handler) addReaction(w http.ResponseWriter, r *http.Request, messageID string) {
	ctx := r.Context()
	aid := agencyID(r)

	var req addReactionRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ParticipantID == "" || req.Emoji == "" {
		writeError(w, http.StatusBadRequest, "participant_id and emoji are required")
		return
	}

	rel, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: aid,
		Name:     "has_reaction",
		FromID:   messageID,
		ToID:     req.ParticipantID,
		Properties: map[string]any{
			"emoji": req.Emoji,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicReactionAdded, ReactionAddedPayload{
		MessageID:     messageID,
		ParticipantID: req.ParticipantID,
		Emoji:         req.Emoji,
	})
	writeJSON(w, http.StatusCreated, rel)
}

// ── (5) MarkRead ─────────────────────────────────────────────────────────────

type markReadRequest struct {
	ParticipantID string `json:"participant_id"`
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request, messageID string) {
	ctx := r.Context()
	aid := agencyID(r)

	var req markReadRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ParticipantID == "" {
		writeError(w, http.StatusBadRequest, "participant_id is required")
		return
	}

	// Idempotent: attempt to create; ignore AlreadyExists.
	rel, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: aid,
		Name:     "read_by",
		FromID:   messageID,
		ToID:     req.ParticipantID,
	})
	if err != nil && !errors.Is(err, entitygraph.ErrEntityAlreadyExists) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

// ── (6) UpdatePresence ────────────────────────────────────────────────────────

type updatePresenceRequest struct {
	Presence string `json:"presence"`
}

func (h *Handler) updatePresence(w http.ResponseWriter, r *http.Request, participantID string) {
	ctx := r.Context()
	aid := agencyID(r)

	var req updatePresenceRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Presence == "" {
		writeError(w, http.StatusBadRequest, "presence is required")
		return
	}

	updated, err := h.dm.UpdateEntity(ctx, aid, participantID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"presence":     req.Presence,
			"last_seen_at": time.Now().UTC().Format(time.RFC3339),
			"updated_at":   time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			writeError(w, http.StatusNotFound, "participant not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicParticipantPresence, ParticipantPresencePayload{
		ParticipantID: participantID,
		Presence:      req.Presence,
	})
	writeJSON(w, http.StatusOK, updated)
}

// ── (7) CreateChannel ─────────────────────────────────────────────────────────

type createChannelRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	EditWindowSeconds *int   `json:"edit_window_seconds,omitempty"`
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	aid := agencyID(r)

	var req createChannelRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	props := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"is_direct":   false,
		"created_at":  time.Now().UTC().Format(time.RFC3339),
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	if req.EditWindowSeconds != nil {
		props["edit_window_seconds"] = *req.EditWindowSeconds
	}

	ch, err := h.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   aid,
		TypeID:     "Channel",
		Properties: props,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

// ── (8) JoinChannel ───────────────────────────────────────────────────────────

type joinChannelRequest struct {
	ParticipantID string `json:"participant_id"`
}

func (h *Handler) joinChannel(w http.ResponseWriter, r *http.Request, channelID string) {
	ctx := r.Context()
	aid := agencyID(r)

	var req joinChannelRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ParticipantID == "" {
		writeError(w, http.StatusBadRequest, "participant_id is required")
		return
	}

	rel, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: aid,
		Name:     "has_member",
		FromID:   channelID,
		ToID:     req.ParticipantID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.publish(ctx, aid, TopicMemberJoined, MemberJoinedPayload{
		ChannelID:     channelID,
		ParticipantID: req.ParticipantID,
	})
	writeJSON(w, http.StatusCreated, rel)
}

// ── (9) CreateDM ─────────────────────────────────────────────────────────────

type createDMRequest struct {
	ParticipantIDs [2]string `json:"participant_ids"`
}

func (h *Handler) createDM(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	aid := agencyID(r)

	var req createDMRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ParticipantIDs[0] == "" || req.ParticipantIDs[1] == "" {
		writeError(w, http.StatusBadRequest, "exactly two participant_ids are required")
		return
	}

	// Idempotent: find existing DM channel between these two participants.
	// For the MVP, create a new DM channel each call and rely on the application
	// layer to deduplicate. A future iteration will check for existing channels.
	now := time.Now().UTC().Format(time.RFC3339)
	ch, err := h.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: aid,
		TypeID:   "Channel",
		Properties: map[string]any{
			"name":       "dm",
			"is_direct":  true,
			"created_at": now,
			"updated_at": now,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, pid := range req.ParticipantIDs {
		if _, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
			AgencyID: aid,
			Name:     "has_member",
			FromID:   ch.ID,
			ToID:     pid,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusCreated, ch)
}
