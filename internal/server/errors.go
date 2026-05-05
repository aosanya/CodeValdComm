package server

import (
	"errors"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toGRPCError maps CodeValdComm domain errors to the appropriate gRPC status code.
// Unknown errors are wrapped as codes.Internal.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, entitygraph.ErrEntityNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, entitygraph.ErrRelationshipNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, entitygraph.ErrSchemaNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, entitygraph.ErrImmutableType):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, codevaldcomm.ErrEditWindowClosed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, codevaldcomm.ErrInvalidEntity):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
