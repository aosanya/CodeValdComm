package codevaldcomm

import (
	"errors"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// Re-exported entitygraph errors — callers check these without importing entitygraph.
var (
	ErrEntityNotFound       = entitygraph.ErrEntityNotFound
	ErrRelationshipNotFound = entitygraph.ErrRelationshipNotFound
	ErrSchemaNotFound       = entitygraph.ErrSchemaNotFound
	ErrImmutableType        = entitygraph.ErrImmutableType
)

// ErrEditWindowClosed is returned when a message edit is attempted outside the
// channel's configured edit window (editWindowSeconds == 0, or the window has elapsed).
var ErrEditWindowClosed = errors.New("edit window closed")

// ErrInvalidEntity is returned when a required entity field fails validation.
var ErrInvalidEntity = errors.New("invalid entity")
