// Package server implements the CommService gRPC handler.
// It wraps a CommSchemaManager and translates between proto messages and domain
// types. No business logic lives here — all calls delegate to the injected manager.
//
// Entity and relationship operations are served by EntityService from SharedLib
// (registered separately in cmd/main.go via server.NewEntityServer).
//
// Files in this package:
//   - server.go        — CommServer struct, constructor, GetSchema handler
//   - entity_server.go — re-export NewEntityServer from SharedLib
//   - errors.go        — gRPC error mapping
package server

import (
	"context"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	pb "github.com/aosanya/CodeValdComm/gen/go/codevaldcomm/v1"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// CommServer implements pb.CommServiceServer by delegating to a CommSchemaManager.
// Construct via New; register with grpc.Server using pb.RegisterCommServiceServer.
type CommServer struct {
	pb.UnimplementedCommServiceServer
	sm codevaldcomm.CommSchemaManager
}

// New constructs a CommServer backed by the given SchemaManager.
func New(sm codevaldcomm.CommSchemaManager) *CommServer {
	return &CommServer{sm: sm}
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

// Compile-time assertion: CommSchemaManager satisfies entitygraph.SchemaManager.
var _ entitygraph.SchemaManager = (codevaldcomm.CommSchemaManager)(nil)
