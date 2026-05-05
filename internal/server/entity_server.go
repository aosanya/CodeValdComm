package server

import (
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	entityserver "github.com/aosanya/CodeValdSharedLib/entitygraph/server"
)

// NewEntityServer constructs an EntityService gRPC server backed by dm.
// Register with pb.RegisterEntityServiceServer in cmd/main.go alongside the
// CommService so that all entity/relationship CRUD is served by the shared
// implementation without duplicating RPCs in the CommService proto.
func NewEntityServer(dm entitygraph.DataManager) *entityserver.EntityServer {
	return entityserver.NewEntityServer(dm)
}
