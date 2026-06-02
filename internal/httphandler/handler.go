// Package httphandler implements HTTP convenience routes for CodeValdComm.
// These routes orchestrate multi-step entity graph operations that cannot be
// expressed as a single EntityService CRUD call.
//
// The twelve flows:
//  1.  SendMessage              POST /channels/{channelId}/messages
//  2.  PromoteToThread          PUT  /messages/{messageId}/promote
//  3.  EditMessage              PUT  /messages/{messageId}
//  4.  AddReaction              POST /messages/{messageId}/reactions
//  5.  MarkRead                 POST /messages/{messageId}/read
//  6.  UpdatePresence           PUT  /participants/{participantId}
//  7.  CreateChannel            POST /channels
//  8.  JoinChannel              POST /channels/{channelId}/members
//  9.  CreateDM                 POST /direct
// 10.  CreateParticipant        POST /participants
// 11.  ListMessages             GET  /channels/{channelId}/messages
// 12.  ListMessagesByWorkflowRun GET  /messages?workflow_run_id=X
package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// Handler implements http.Handler for the comm convenience flows.
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
// Handles both direct calls (/channels/...) and Cross-forwarded calls
// (/{agencyId}/comm/channels/...). When Cross forwards, the full original
// path is preserved; the agency ID is parsed from the path and injected
// as X-Agency-Id so agencyID(r) can read it.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	allParts := strings.Split(path, "/")

	// Path form from Cross: {agencyId}/comm/{resource...}
	if len(allParts) >= 2 && allParts[1] == "comm" {
		r.Header.Set("X-Agency-Id", allParts[0])
		path = strings.Join(allParts[2:], "/")
	}

	parts := strings.SplitN(path, "/", 4)

	switch {
	case r.Method == http.MethodPost && len(parts) >= 1 && parts[0] == "channels" && len(parts) == 1:
		h.createChannel(w, r)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "channels" && parts[2] == "messages":
		h.sendMessage(w, r, parts[1])
	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "channels" && parts[2] == "messages":
		h.listMessages(w, r, parts[1])
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "messages":
		h.listMessagesByWorkflowRun(w, r)
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
	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "participants":
		h.createParticipant(w, r)
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
