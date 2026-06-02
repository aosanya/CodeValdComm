package httphandler

import (
	"net/http"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── (7) CreateChannel ─────────────────────────────────────────────────────────

type createChannelRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	CreatedBy         string `json:"created_by,omitempty"`
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
	if req.CreatedBy != "" {
		props["created_by"] = req.CreatedBy
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

	h.publish(ctx, aid, TopicMemberJoined, map[string]string{"channel_id": channelID, "participant_id": req.ParticipantID})
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
