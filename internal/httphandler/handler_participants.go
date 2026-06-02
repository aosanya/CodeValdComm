package httphandler

import (
	"errors"
	"net/http"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

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

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             rel.ID,
		"message_id":     messageID,
		"participant_id": req.ParticipantID,
		"emoji":          req.Emoji,
	})
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

	now := time.Now().UTC().Format(time.RFC3339)
	rel, err := h.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: aid,
		Name:     "read_by",
		FromID:   messageID,
		ToID:     req.ParticipantID,
		Properties: map[string]any{
			"read_at": now,
		},
	})
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityAlreadyExists) {
			writeJSON(w, http.StatusOK, map[string]any{
				"message_id":     messageID,
				"participant_id": req.ParticipantID,
				"read_at":        now,
			})
			return
		}
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

	writeJSON(w, http.StatusOK, updated)
}

// ── (10) CreateParticipant ────────────────────────────────────────────────────

type createParticipantRequest struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Presence    string `json:"presence"`
}

func (h *Handler) createParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	aid := agencyID(r)

	var req createParticipantRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	p, err := h.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: aid,
		TypeID:   "Participant",
		Properties: map[string]any{
			"user_id":      req.UserID,
			"display_name": req.DisplayName,
			"presence":     req.Presence,
			"created_at":   now,
			"updated_at":   now,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}
