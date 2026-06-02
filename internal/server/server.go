// Package server implements the CommService gRPC handler.
// It wraps a CommSchemaManager and translates between proto messages and domain
// types. No business logic lives here — all calls delegate to the injected manager.
//
// Entity and relationship operations are served by EntityService from SharedLib
// (registered separately in cmd/main.go via server.NewEntityServer).
//
// Files in this package:
//   - server.go        — CommServer struct, constructor, GetSchema + RollbackByWorkflowRun handlers
//   - entity_server.go — re-export NewEntityServer from SharedLib
//   - errors.go        — gRPC error mapping
package server

import (
	"context"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	pb "github.com/aosanya/CodeValdComm/gen/go/codevaldcomm/v1"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// CommServer implements pb.CommServiceServer by delegating to a CommSchemaManager
// for GetSchema and to a CommDataManager + CrossPublisher for the rollback leg.
// Construct via New; register with grpc.Server using pb.RegisterCommServiceServer.
type CommServer struct {
	pb.UnimplementedCommServiceServer
	dm  codevaldcomm.CommDataManager
	sm  codevaldcomm.CommSchemaManager
	pub codevaldcomm.CrossPublisher
}

// New constructs a CommServer backed by the given DataManager, SchemaManager,
// and (optionally nil) publisher. dm may be nil only if no entity-mutating
// RPCs are exercised (GetSchema works with a nil dm).
func New(dm codevaldcomm.CommDataManager, sm codevaldcomm.CommSchemaManager, pub codevaldcomm.CrossPublisher) *CommServer {
	return &CommServer{dm: dm, sm: sm, pub: pub}
}

// GetSchema returns the active comm schema for the given agency.
// Returns NOT_FOUND if no schema has been seeded for the agency yet.
func (s *CommServer) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	schema, err := s.sm.GetActive(ctx, req.GetAgencyId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetSchemaResponse{
		Id:       schema.ID,
		Version:  int32(schema.Version),
		Tag:      schema.Tag,
		AgencyId: schema.AgencyID,
	}, nil
}

// RollbackByWorkflowRun implements pb.CommServiceServer (FEAT-20260602-004
// Phase 2 — Comm leg). It posts one synthetic "pipeline rolled back" follow-up
// Message into every channel that holds at least one Message tagged with the
// rolled-back workflow_run_id, skipping channels that already received a
// notification for this run.
func (s *CommServer) RollbackByWorkflowRun(ctx context.Context, req *pb.RollbackByWorkflowRunRequest) (*pb.RollbackByWorkflowRunResponse, error) {
	result, err := codevaldcomm.RollbackByWorkflowRun(
		ctx, s.dm, s.pub,
		req.GetAgencyId(), req.GetWorkflowRunId(), req.GetReason(),
	)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.RollbackByWorkflowRunResponse{
		WorkflowRunId:          result.WorkflowRunID,
		NotifiedChannelIds:     result.NotifiedChannelIDs,
		SkippedChannelIds:      result.SkippedChannelIDs,
		NotificationMessageIds: result.NotificationMessageIDs,
	}, nil
}

// Compile-time assertion: CommSchemaManager satisfies entitygraph.SchemaManager.
var _ entitygraph.SchemaManager = (codevaldcomm.CommSchemaManager)(nil)
