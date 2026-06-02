package server_test

import (
	"context"
	"errors"
	"testing"

	pb "github.com/aosanya/CodeValdComm/gen/go/codevaldcomm/v1"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// rollbackFakeDM is a minimal DataManager double for the rollback RPC tests.
// Behaviour is shaped to cover the three RPC outcomes: success, idempotency,
// and propagated storage error → INTERNAL.
type rollbackFakeDM struct {
	messages         []entitygraph.Entity
	hasMessageEdges  []entitygraph.Relationship
	failListEntities error
}

func (f *rollbackFakeDM) CreateEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	ent := entitygraph.Entity{ID: "new-msg", TypeID: req.TypeID, Properties: req.Properties}
	if req.TypeID == "Message" {
		f.messages = append(f.messages, ent)
	}
	return ent, nil
}
func (f *rollbackFakeDM) GetEntity(_ context.Context, _, _ string) (entitygraph.Entity, error) {
	return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
}
func (f *rollbackFakeDM) UpdateEntity(_ context.Context, _, _ string, _ entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{}, nil
}
func (f *rollbackFakeDM) DeleteEntity(_ context.Context, _, _ string) error { return nil }
func (f *rollbackFakeDM) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	if f.failListEntities != nil {
		return nil, f.failListEntities
	}
	wrid, _ := filter.Properties["workflow_run_id"].(string)
	out := []entitygraph.Entity{}
	for _, m := range f.messages {
		if v, _ := m.Properties["workflow_run_id"].(string); v == wrid {
			out = append(out, m)
		}
	}
	return out, nil
}
func (f *rollbackFakeDM) UpsertEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{ID: "u", TypeID: req.TypeID, Properties: req.Properties}, nil
}
func (f *rollbackFakeDM) CreateRelationship(_ context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	rel := entitygraph.Relationship{ID: "rel-x", Name: req.Name, FromID: req.FromID, ToID: req.ToID}
	if req.Name == "has_message" {
		f.hasMessageEdges = append(f.hasMessageEdges, rel)
	}
	return rel, nil
}
func (f *rollbackFakeDM) GetRelationship(_ context.Context, _, relID string) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{ID: relID}, nil
}
func (f *rollbackFakeDM) DeleteRelationship(_ context.Context, _, _ string) error { return nil }
func (f *rollbackFakeDM) ListRelationships(_ context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	out := []entitygraph.Relationship{}
	for _, r := range f.hasMessageEdges {
		if filter.Name != "" && r.Name != filter.Name {
			continue
		}
		if filter.ToID != "" && r.ToID != filter.ToID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
func (f *rollbackFakeDM) TraverseGraph(_ context.Context, _ entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	return entitygraph.TraverseGraphResult{}, nil
}

// seedMessage drops a Message + has_message edge into the fake.
func (f *rollbackFakeDM) seedMessage(msgID, channelID, workflowRunID string) {
	f.messages = append(f.messages, entitygraph.Entity{
		ID: msgID, TypeID: "Message",
		Properties: map[string]any{
			"body": "b", "sender_id": "u1", "workflow_run_id": workflowRunID,
		},
	})
	f.hasMessageEdges = append(f.hasMessageEdges, entitygraph.Relationship{
		ID: "e-" + msgID, Name: "has_message", FromID: channelID, ToID: msgID,
	})
}

func TestRollbackByWorkflowRun_RPC_Success(t *testing.T) {
	dm := &rollbackFakeDM{}
	dm.seedMessage("m1", "ch-1", "run-42")
	dm.seedMessage("m2", "ch-2", "run-42")

	client := newTestClient(t, &fakeSchemaManager{}, withDataManager(dm))

	resp, err := client.RollbackByWorkflowRun(context.Background(), &pb.RollbackByWorkflowRunRequest{
		AgencyId:      "agency-1",
		WorkflowRunId: "run-42",
		Reason:        "merge regressed prod",
	})
	if err != nil {
		t.Fatalf("RollbackByWorkflowRun: %v", err)
	}
	if resp.WorkflowRunId != "run-42" {
		t.Errorf("WorkflowRunId = %q, want run-42", resp.WorkflowRunId)
	}
	if len(resp.NotifiedChannelIds) != 2 {
		t.Errorf("NotifiedChannelIds = %v, want 2 entries", resp.NotifiedChannelIds)
	}
	if len(resp.NotificationMessageIds) != 2 {
		t.Errorf("NotificationMessageIds = %v, want 2 entries", resp.NotificationMessageIds)
	}
	if len(resp.SkippedChannelIds) != 0 {
		t.Errorf("SkippedChannelIds = %v, want none", resp.SkippedChannelIds)
	}
}

func TestRollbackByWorkflowRun_RPC_EmptyWorkflowRunID_InvalidArgument(t *testing.T) {
	client := newTestClient(t, &fakeSchemaManager{}, withDataManager(&rollbackFakeDM{}))

	_, err := client.RollbackByWorkflowRun(context.Background(), &pb.RollbackByWorkflowRunRequest{
		AgencyId: "agency-1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestRollbackByWorkflowRun_RPC_StorageError_Internal(t *testing.T) {
	dm := &rollbackFakeDM{failListEntities: errors.New("arango unreachable")}
	client := newTestClient(t, &fakeSchemaManager{}, withDataManager(dm))

	_, err := client.RollbackByWorkflowRun(context.Background(), &pb.RollbackByWorkflowRunRequest{
		AgencyId:      "agency-1",
		WorkflowRunId: "run-42",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.Internal {
		t.Errorf("status code = %v, want %v", code, codes.Internal)
	}
}
