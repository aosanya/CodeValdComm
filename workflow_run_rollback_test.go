package codevaldcomm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

// fakeDM is a minimal in-memory DataManager double for the rollback tests.
// It records mutations so tests can assert on them and supports the queries
// RollbackByWorkflowRun makes: ListEntities by workflow_run_id, and
// ListRelationships filtered by has_message + ToID.
type fakeDM struct {
	messages         []entitygraph.Entity
	hasMessageEdges  []entitygraph.Relationship // FromID=channel, ToID=message
	createdEntities  []entitygraph.CreateEntityRequest
	createdRels      []entitygraph.CreateRelationshipRequest
	nextID           int
	failListEntities error
	failListRels     error
	failCreate       error
}

func (f *fakeDM) CreateEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	if f.failCreate != nil {
		return entitygraph.Entity{}, f.failCreate
	}
	f.nextID++
	id := nextID(req.TypeID, f.nextID)
	f.createdEntities = append(f.createdEntities, req)
	ent := entitygraph.Entity{ID: id, TypeID: req.TypeID, Properties: req.Properties}
	// Track rollback notification messages so a second call sees them.
	if req.TypeID == "Message" {
		f.messages = append(f.messages, ent)
	}
	return ent, nil
}

func (f *fakeDM) GetEntity(_ context.Context, _, entityID string) (entitygraph.Entity, error) {
	for _, m := range f.messages {
		if m.ID == entityID {
			return m, nil
		}
	}
	return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
}

func (f *fakeDM) UpdateEntity(_ context.Context, _, _ string, _ entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{}, nil
}
func (f *fakeDM) DeleteEntity(_ context.Context, _, _ string) error { return nil }

func (f *fakeDM) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	if f.failListEntities != nil {
		return nil, f.failListEntities
	}
	wrid, _ := filter.Properties["workflow_run_id"].(string)
	out := make([]entitygraph.Entity, 0, len(f.messages))
	for _, m := range f.messages {
		if filter.TypeID != "" && m.TypeID != filter.TypeID {
			continue
		}
		if wrid != "" {
			v, _ := m.Properties["workflow_run_id"].(string)
			if v != wrid {
				continue
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeDM) UpsertEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{ID: "upsert-1", TypeID: req.TypeID, Properties: req.Properties}, nil
}

func (f *fakeDM) CreateRelationship(_ context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	f.nextID++
	id := nextID("rel", f.nextID)
	f.createdRels = append(f.createdRels, req)
	rel := entitygraph.Relationship{ID: id, Name: req.Name, FromID: req.FromID, ToID: req.ToID}
	if req.Name == "has_message" {
		f.hasMessageEdges = append(f.hasMessageEdges, rel)
	}
	return rel, nil
}

func (f *fakeDM) GetRelationship(_ context.Context, _, relID string) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{ID: relID}, nil
}
func (f *fakeDM) DeleteRelationship(_ context.Context, _, _ string) error { return nil }

func (f *fakeDM) ListRelationships(_ context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	if f.failListRels != nil {
		return nil, f.failListRels
	}
	out := make([]entitygraph.Relationship, 0)
	for _, r := range f.hasMessageEdges {
		if filter.Name != "" && r.Name != filter.Name {
			continue
		}
		if filter.ToID != "" && r.ToID != filter.ToID {
			continue
		}
		if filter.FromID != "" && r.FromID != filter.FromID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeDM) TraverseGraph(_ context.Context, _ entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	return entitygraph.TraverseGraphResult{}, nil
}

// seedMessage adds a pre-existing Message + its has_message edge to f.
func (f *fakeDM) seedMessage(msgID, channelID, workflowRunID string, isRollback bool) {
	props := map[string]any{
		"body":            "body",
		"sender_id":       "u1",
		"workflow_run_id": workflowRunID,
	}
	if isRollback {
		props["rollback_notification"] = true
	}
	f.messages = append(f.messages, entitygraph.Entity{
		ID:         msgID,
		TypeID:     "Message",
		Properties: props,
	})
	f.hasMessageEdges = append(f.hasMessageEdges, entitygraph.Relationship{
		ID:     "edge-" + msgID,
		Name:   "has_message",
		FromID: channelID,
		ToID:   msgID,
	})
}

func nextID(prefix string, n int) string {
	return prefix + "-" + itoa(n)
}

// itoa avoids pulling strconv into the file just for this.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

type recordingPublisher struct{ events []eventbus.Event }

func (r *recordingPublisher) Publish(_ context.Context, e eventbus.Event) error {
	r.events = append(r.events, e)
	return nil
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestRollbackByWorkflowRun_EmptyWorkflowRunID(t *testing.T) {
	_, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), &fakeDM{}, nil, "agency-1", "", "")
	if !errors.Is(err, codevaldcomm.ErrWorkflowRunIDRequired) {
		t.Fatalf("error = %v, want ErrWorkflowRunIDRequired", err)
	}
}

func TestRollbackByWorkflowRun_NoMatchingMessages_EmptyResult(t *testing.T) {
	dm := &fakeDM{}
	pub := &recordingPublisher{}
	res, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, pub, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(res.NotifiedChannelIDs) != 0 || len(res.SkippedChannelIDs) != 0 || len(res.NotificationMessageIDs) != 0 {
		t.Errorf("expected empty result, got %+v", res)
	}
	if len(pub.events) != 0 {
		t.Errorf("expected no events, got %d", len(pub.events))
	}
	if len(dm.createdEntities) != 0 {
		t.Errorf("expected no messages created, got %d", len(dm.createdEntities))
	}
}

func TestRollbackByWorkflowRun_SingleChannel_PostsNotification(t *testing.T) {
	dm := &fakeDM{}
	dm.seedMessage("msg-A", "ch-1", "run-42", false)
	dm.seedMessage("msg-B", "ch-1", "run-42", false) // same channel, second message
	pub := &recordingPublisher{}

	res, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, pub, "agency-1", "run-42", "regression")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}

	if got, want := len(res.NotifiedChannelIDs), 1; got != want {
		t.Fatalf("NotifiedChannelIDs len = %d, want %d", got, want)
	}
	if res.NotifiedChannelIDs[0] != "ch-1" {
		t.Errorf("NotifiedChannelIDs[0] = %q, want ch-1", res.NotifiedChannelIDs[0])
	}
	if len(res.SkippedChannelIDs) != 0 {
		t.Errorf("SkippedChannelIDs should be empty, got %v", res.SkippedChannelIDs)
	}
	if len(res.NotificationMessageIDs) != 1 {
		t.Fatalf("NotificationMessageIDs len = %d, want 1", len(res.NotificationMessageIDs))
	}

	// One new Message entity + one has_message edge created.
	if len(dm.createdEntities) != 1 {
		t.Fatalf("createdEntities = %d, want 1", len(dm.createdEntities))
	}
	created := dm.createdEntities[0]
	if created.TypeID != "Message" {
		t.Errorf("created TypeID = %q, want Message", created.TypeID)
	}
	if v, _ := created.Properties["rollback_notification"].(bool); !v {
		t.Errorf("rollback_notification = %v, want true", created.Properties["rollback_notification"])
	}
	if v, _ := created.Properties["workflow_run_id"].(string); v != "run-42" {
		t.Errorf("workflow_run_id = %q, want run-42", v)
	}
	if v, _ := created.Properties["sender_id"].(string); v != codevaldcomm.RollbackSenderID {
		t.Errorf("sender_id = %q, want %q", v, codevaldcomm.RollbackSenderID)
	}
	if v, _ := created.Properties["rollback_reason"].(string); v != "regression" {
		t.Errorf("rollback_reason = %q, want regression", v)
	}
	if body, _ := created.Properties["body"].(string); !strings.Contains(body, "run-42") || !strings.Contains(body, "regression") {
		t.Errorf("body = %q, want it to mention run-42 and reason", body)
	}

	if len(dm.createdRels) != 1 || dm.createdRels[0].Name != "has_message" || dm.createdRels[0].FromID != "ch-1" {
		t.Errorf("createdRels = %+v, want one has_message from ch-1", dm.createdRels)
	}

	if len(pub.events) != 1 || pub.events[0].Topic != codevaldcomm.TopicPipelineRolledBack {
		t.Fatalf("events = %+v, want one TopicPipelineRolledBack", pub.events)
	}
	payload, ok := pub.events[0].Payload.(codevaldcomm.PipelineRolledBackPayload)
	if !ok {
		t.Fatalf("payload type = %T, want PipelineRolledBackPayload", pub.events[0].Payload)
	}
	if payload.ChannelID != "ch-1" || payload.WorkflowRunID != "run-42" || payload.Reason != "regression" {
		t.Errorf("payload = %+v, want ch-1/run-42/regression", payload)
	}
	if payload.MessageID != res.NotificationMessageIDs[0] {
		t.Errorf("payload.MessageID = %q, want %q (res.NotificationMessageIDs[0])", payload.MessageID, res.NotificationMessageIDs[0])
	}
}

func TestRollbackByWorkflowRun_MultipleChannels_OneNotificationPerChannel(t *testing.T) {
	dm := &fakeDM{}
	dm.seedMessage("msg-A", "ch-1", "run-42", false)
	dm.seedMessage("msg-B", "ch-2", "run-42", false)
	dm.seedMessage("msg-C", "ch-2", "run-42", false) // second msg in ch-2 — still only one notification
	dm.seedMessage("msg-D", "ch-3", "other-run", false) // unrelated run — must be ignored
	pub := &recordingPublisher{}

	res, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, pub, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}

	if got, want := len(res.NotifiedChannelIDs), 2; got != want {
		t.Fatalf("NotifiedChannelIDs len = %d, want %d", got, want)
	}
	seen := map[string]bool{}
	for _, c := range res.NotifiedChannelIDs {
		seen[c] = true
	}
	if !seen["ch-1"] || !seen["ch-2"] {
		t.Errorf("NotifiedChannelIDs = %v, want ch-1 and ch-2", res.NotifiedChannelIDs)
	}
	if seen["ch-3"] {
		t.Error("ch-3 (different run) should not be notified")
	}
	if len(pub.events) != 2 {
		t.Errorf("events = %d, want 2", len(pub.events))
	}
}

func TestRollbackByWorkflowRun_AlreadyNotified_Skipped(t *testing.T) {
	dm := &fakeDM{}
	dm.seedMessage("msg-A", "ch-1", "run-42", false)
	dm.seedMessage("msg-rollback", "ch-1", "run-42", true) // existing rollback notification
	pub := &recordingPublisher{}

	res, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, pub, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(res.NotifiedChannelIDs) != 0 {
		t.Errorf("NotifiedChannelIDs should be empty, got %v", res.NotifiedChannelIDs)
	}
	if got, want := len(res.SkippedChannelIDs), 1; got != want || res.SkippedChannelIDs[0] != "ch-1" {
		t.Errorf("SkippedChannelIDs = %v, want [ch-1]", res.SkippedChannelIDs)
	}
	if len(pub.events) != 0 {
		t.Errorf("no events expected on skip; got %d", len(pub.events))
	}
	if len(dm.createdEntities) != 0 {
		t.Errorf("no messages should be created on skip; got %d", len(dm.createdEntities))
	}
}

func TestRollbackByWorkflowRun_IdempotentRetry(t *testing.T) {
	dm := &fakeDM{}
	dm.seedMessage("msg-A", "ch-1", "run-42", false)
	pub := &recordingPublisher{}

	// First call posts the notification.
	res1, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, pub, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("first RollbackByWorkflowRun: %v", err)
	}
	if len(res1.NotifiedChannelIDs) != 1 {
		t.Fatalf("first call NotifiedChannelIDs = %v", res1.NotifiedChannelIDs)
	}

	// Second call must skip ch-1 because the notification posted above is now visible.
	res2, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, pub, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("second RollbackByWorkflowRun: %v", err)
	}
	if len(res2.NotifiedChannelIDs) != 0 || len(res2.SkippedChannelIDs) != 1 || res2.SkippedChannelIDs[0] != "ch-1" {
		t.Errorf("second call result = %+v, want only ch-1 skipped", res2)
	}
	// Only one event total — first call only.
	if len(pub.events) != 1 {
		t.Errorf("events across both calls = %d, want 1", len(pub.events))
	}
}

func TestRollbackByWorkflowRun_OrphanMessage_NoCrash(t *testing.T) {
	// A message tagged with the run but with no owning channel edge.
	dm := &fakeDM{}
	dm.messages = append(dm.messages, entitygraph.Entity{
		ID:     "orphan",
		TypeID: "Message",
		Properties: map[string]any{
			"workflow_run_id": "run-42",
		},
	})

	res, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, nil, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(res.NotifiedChannelIDs) != 0 || len(res.SkippedChannelIDs) != 0 {
		t.Errorf("orphan message should produce empty result, got %+v", res)
	}
}

func TestRollbackByWorkflowRun_NilPublisher_NoPanic(t *testing.T) {
	dm := &fakeDM{}
	dm.seedMessage("msg-A", "ch-1", "run-42", false)

	res, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, nil, "agency-1", "run-42", "")
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if len(res.NotifiedChannelIDs) != 1 {
		t.Errorf("NotifiedChannelIDs = %v, want one channel", res.NotifiedChannelIDs)
	}
}

func TestRollbackByWorkflowRun_ListEntitiesError_Propagated(t *testing.T) {
	dm := &fakeDM{failListEntities: errors.New("arango unreachable")}
	_, err := codevaldcomm.RollbackByWorkflowRun(context.Background(), dm, nil, "agency-1", "run-42", "")
	if err == nil || !strings.Contains(err.Error(), "arango unreachable") {
		t.Errorf("err = %v, want it to wrap the storage error", err)
	}
}

func TestAllTopics_IncludesPipelineRolledBack(t *testing.T) {
	for _, topic := range codevaldcomm.AllTopics() {
		if topic == codevaldcomm.TopicPipelineRolledBack {
			return
		}
	}
	t.Errorf("AllTopics() missing %q", codevaldcomm.TopicPipelineRolledBack)
}
