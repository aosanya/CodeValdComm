package httphandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdComm/internal/httphandler"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// ── fakeDataManager ───────────────────────────────────────────────────────────

type fakeDataManager struct {
	createEntity      func(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error)
	getEntity         func(ctx context.Context, agencyID, entityID string) (entitygraph.Entity, error)
	updateEntity      func(ctx context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error)
	deleteEntity      func(ctx context.Context, agencyID, entityID string) error
	listEntities      func(ctx context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error)
	upsertEntity      func(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error)
	createRelationship func(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error)
	getRelationship    func(ctx context.Context, agencyID, relID string) (entitygraph.Relationship, error)
	deleteRelationship func(ctx context.Context, agencyID, relID string) error
	listRelationships  func(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error)
	traverseGraph      func(ctx context.Context, req entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error)
}

func (f *fakeDataManager) CreateEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	if f.createEntity != nil {
		return f.createEntity(ctx, req)
	}
	return entitygraph.Entity{ID: "entity-1", TypeID: req.TypeID, Properties: req.Properties}, nil
}
func (f *fakeDataManager) GetEntity(ctx context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	if f.getEntity != nil {
		return f.getEntity(ctx, agencyID, entityID)
	}
	return entitygraph.Entity{ID: entityID, TypeID: "Message", Properties: map[string]any{"body": "hello", "sender_id": "u1"}}, nil
}
func (f *fakeDataManager) UpdateEntity(ctx context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	if f.updateEntity != nil {
		return f.updateEntity(ctx, agencyID, entityID, req)
	}
	return entitygraph.Entity{ID: entityID, Properties: req.Properties}, nil
}
func (f *fakeDataManager) DeleteEntity(ctx context.Context, agencyID, entityID string) error {
	if f.deleteEntity != nil {
		return f.deleteEntity(ctx, agencyID, entityID)
	}
	return nil
}
func (f *fakeDataManager) ListEntities(ctx context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	if f.listEntities != nil {
		return f.listEntities(ctx, filter)
	}
	return nil, nil
}
func (f *fakeDataManager) UpsertEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	if f.upsertEntity != nil {
		return f.upsertEntity(ctx, req)
	}
	return entitygraph.Entity{ID: "entity-1", TypeID: req.TypeID, Properties: req.Properties}, nil
}
func (f *fakeDataManager) CreateRelationship(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	if f.createRelationship != nil {
		return f.createRelationship(ctx, req)
	}
	return entitygraph.Relationship{ID: "rel-1", Name: req.Name, FromID: req.FromID, ToID: req.ToID}, nil
}
func (f *fakeDataManager) GetRelationship(ctx context.Context, agencyID, relID string) (entitygraph.Relationship, error) {
	if f.getRelationship != nil {
		return f.getRelationship(ctx, agencyID, relID)
	}
	return entitygraph.Relationship{ID: relID}, nil
}
func (f *fakeDataManager) DeleteRelationship(ctx context.Context, agencyID, relID string) error {
	if f.deleteRelationship != nil {
		return f.deleteRelationship(ctx, agencyID, relID)
	}
	return nil
}
func (f *fakeDataManager) ListRelationships(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	if f.listRelationships != nil {
		return f.listRelationships(ctx, filter)
	}
	return nil, nil
}
func (f *fakeDataManager) TraverseGraph(ctx context.Context, req entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	if f.traverseGraph != nil {
		return f.traverseGraph(ctx, req)
	}
	return entitygraph.TraverseGraphResult{}, nil
}

// fakePublisher records events for assertion.
type fakePublisher struct {
	events []eventbus.Event
}

func (p *fakePublisher) Publish(_ context.Context, e eventbus.Event) error {
	p.events = append(p.events, e)
	return nil
}

func newHandler(dm *fakeDataManager, pub eventbus.Publisher) *httphandler.Handler {
	return httphandler.New(dm, nil, pub)
}

func bodyJSON(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func withAgency(r *http.Request, agencyID string) *http.Request {
	r.Header.Set("X-Agency-Id", agencyID)
	return r
}

// ── SendMessage ───────────────────────────────────────────────────────────────

func TestSendMessage_Success(t *testing.T) {
	var createdType string
	dm := &fakeDataManager{
		createEntity: func(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
			createdType = req.TypeID
			return entitygraph.Entity{ID: "msg-1", TypeID: req.TypeID, Properties: req.Properties}, nil
		},
	}
	pub := &fakePublisher{}
	h := newHandler(dm, pub)

	body := bodyJSON(map[string]string{"body": "hello", "sender_id": "user-1"})
	r := withAgency(httptest.NewRequest(http.MethodPost, "/channels/ch-1/messages", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if createdType != "Message" {
		t.Errorf("created TypeID = %q, want %q", createdType, "Message")
	}
	if len(pub.events) != 1 || pub.events[0].Topic != httphandler.TopicMessageSent {
		t.Errorf("expected TopicMessageSent event; got %v", pub.events)
	}
}

func TestSendMessage_MissingBody(t *testing.T) {
	h := newHandler(&fakeDataManager{}, nil)
	body := bodyJSON(map[string]string{"sender_id": "u1"}) // missing body
	r := withAgency(httptest.NewRequest(http.MethodPost, "/channels/ch-1/messages", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ── PromoteToThread ───────────────────────────────────────────────────────────

func TestPromoteToThread_Idempotent(t *testing.T) {
	dm := &fakeDataManager{
		getEntity: func(_ context.Context, _, _ string) (entitygraph.Entity, error) {
			return entitygraph.Entity{
				ID:         "msg-1",
				TypeID:     "Message",
				Properties: map[string]any{"is_thread_root": true},
			}, nil
		},
	}
	var updateCalled bool
	dm.updateEntity = func(_ context.Context, _, _ string, _ entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
		updateCalled = true
		return entitygraph.Entity{}, nil
	}
	h := newHandler(dm, nil)

	r := withAgency(httptest.NewRequest(http.MethodPut, "/messages/msg-1/promote", nil), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if updateCalled {
		t.Error("UpdateEntity should not be called when message is already a thread root")
	}
}

func TestPromoteToThread_Success(t *testing.T) {
	dm := &fakeDataManager{
		getEntity: func(_ context.Context, _, id string) (entitygraph.Entity, error) {
			return entitygraph.Entity{
				ID:         id,
				TypeID:     "Message",
				Properties: map[string]any{"is_thread_root": false},
			}, nil
		},
	}
	pub := &fakePublisher{}
	h := newHandler(dm, pub)

	r := withAgency(httptest.NewRequest(http.MethodPut, "/messages/msg-2/promote", nil), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(pub.events) != 1 || pub.events[0].Topic != httphandler.TopicThreadPromoted {
		t.Errorf("expected TopicThreadPromoted; got %v", pub.events)
	}
}

// ── EditMessage ───────────────────────────────────────────────────────────────

func TestEditMessage_EditWindowClosed(t *testing.T) {
	// Channel with editWindowSeconds = 0 (editing disabled).
	dm := &fakeDataManager{
		getEntity: func(_ context.Context, _, id string) (entitygraph.Entity, error) {
			return entitygraph.Entity{
				ID:         id,
				TypeID:     "Message",
				Properties: map[string]any{"body": "old"},
			}, nil
		},
		listRelationships: func(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
			return []entitygraph.Relationship{{ID: "rel-1", Name: "belongs_to_channel", ToID: "ch-1"}}, nil
		},
	}
	// Provide a second getEntity for the channel.
	callCount := 0
	dm.getEntity = func(_ context.Context, _, id string) (entitygraph.Entity, error) {
		callCount++
		if callCount == 1 {
			// First call: the message itself.
			return entitygraph.Entity{ID: id, TypeID: "Message", Properties: map[string]any{"body": "old"}}, nil
		}
		// Second call: the channel.
		return entitygraph.Entity{
			ID:         id,
			TypeID:     "Channel",
			Properties: map[string]any{"edit_window_seconds": float64(0)},
		}, nil
	}

	h := newHandler(dm, nil)
	body := bodyJSON(map[string]string{"body": "new text", "editor_id": "u1"})
	r := withAgency(httptest.NewRequest(http.MethodPut, "/messages/msg-1", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (edit window closed)", w.Code, http.StatusForbidden)
	}
}

func TestEditMessage_WithinWindow(t *testing.T) {
	createdAt := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
	callCount := 0
	dm := &fakeDataManager{
		getEntity: func(_ context.Context, _, id string) (entitygraph.Entity, error) {
			callCount++
			if callCount == 1 {
				return entitygraph.Entity{
					ID:     id,
					TypeID: "Message",
					Properties: map[string]any{
						"body":       "old",
						"created_at": createdAt,
					},
				}, nil
			}
			return entitygraph.Entity{
				ID:         id,
				TypeID:     "Channel",
				Properties: map[string]any{"edit_window_seconds": float64(300)},
			}, nil
		},
		listRelationships: func(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
			return []entitygraph.Relationship{{ID: "rel-1", Name: "belongs_to_channel", ToID: "ch-1"}}, nil
		},
	}
	pub := &fakePublisher{}
	h := newHandler(dm, pub)

	body := bodyJSON(map[string]string{"body": "updated", "editor_id": "u1"})
	r := withAgency(httptest.NewRequest(http.MethodPut, "/messages/msg-1", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(pub.events) != 1 || pub.events[0].Topic != httphandler.TopicMessageEdited {
		t.Errorf("expected TopicMessageEdited; got %v", pub.events)
	}
}

func TestEditMessage_TimedOut(t *testing.T) {
	// Message created 400 seconds ago; window is 300 seconds.
	createdAt := time.Now().UTC().Add(-400 * time.Second).Format(time.RFC3339)
	callCount := 0
	dm := &fakeDataManager{
		getEntity: func(_ context.Context, _, id string) (entitygraph.Entity, error) {
			callCount++
			if callCount == 1 {
				return entitygraph.Entity{
					ID:     id,
					TypeID: "Message",
					Properties: map[string]any{
						"body":       "old",
						"created_at": createdAt,
					},
				}, nil
			}
			return entitygraph.Entity{
				ID:         id,
				TypeID:     "Channel",
				Properties: map[string]any{"edit_window_seconds": float64(300)},
			}, nil
		},
		listRelationships: func(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
			return []entitygraph.Relationship{{ID: "rel-1", Name: "belongs_to_channel", ToID: "ch-1"}}, nil
		},
	}

	h := newHandler(dm, nil)
	body := bodyJSON(map[string]string{"body": "late edit"})
	r := withAgency(httptest.NewRequest(http.MethodPut, "/messages/msg-1", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (edit timed out)", w.Code, http.StatusForbidden)
	}
}

// ── CreateChannel ─────────────────────────────────────────────────────────────

func TestCreateChannel_Success(t *testing.T) {
	h := newHandler(&fakeDataManager{}, nil)
	body := bodyJSON(map[string]string{"name": "general"})
	r := withAgency(httptest.NewRequest(http.MethodPost, "/channels", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

// ── JoinChannel ───────────────────────────────────────────────────────────────

func TestJoinChannel_Success(t *testing.T) {
	pub := &fakePublisher{}
	h := newHandler(&fakeDataManager{}, pub)

	body := bodyJSON(map[string]string{"participant_id": "p-1"})
	r := withAgency(httptest.NewRequest(http.MethodPost, "/channels/ch-1/members", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if len(pub.events) != 1 || pub.events[0].Topic != httphandler.TopicMemberJoined {
		t.Errorf("expected TopicMemberJoined; got %v", pub.events)
	}
}

// ── EditMessage always-editable window (-1) ───────────────────────────────────

func TestEditMessage_AlwaysEditable(t *testing.T) {
	callCount := 0
	dm := &fakeDataManager{
		getEntity: func(_ context.Context, _, id string) (entitygraph.Entity, error) {
			callCount++
			if callCount == 1 {
				return entitygraph.Entity{
					ID:     id,
					TypeID: "Message",
					Properties: map[string]any{
						"body":       "old",
						"created_at": time.Now().UTC().Add(-9999 * time.Second).Format(time.RFC3339),
					},
				}, nil
			}
			return entitygraph.Entity{
				ID:         id,
				TypeID:     "Channel",
				Properties: map[string]any{"edit_window_seconds": float64(-1)},
			}, nil
		},
		listRelationships: func(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
			return []entitygraph.Relationship{{ID: "rel-1", Name: "belongs_to_channel", ToID: "ch-1"}}, nil
		},
	}
	h := newHandler(dm, nil)

	body := bodyJSON(map[string]string{"body": "any time edit"})
	r := withAgency(httptest.NewRequest(http.MethodPut, "/messages/msg-1", body), "agency-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (always editable)", w.Code, http.StatusOK)
	}
}

// ── AllTopics ─────────────────────────────────────────────────────────────────

func TestAllTopics(t *testing.T) {
	topics := httphandler.AllTopics()
	if len(topics) == 0 {
		t.Error("AllTopics: expected non-empty list")
	}
	seen := make(map[string]bool)
	for _, topic := range topics {
		if seen[topic] {
			t.Errorf("duplicate topic: %q", topic)
		}
		seen[topic] = true
	}
}

// Ensure Handler compiles with a nil CrossPublisher (eventbus.Publisher).
var _ codevaldcomm.CrossPublisher = (*fakePublisher)(nil)
