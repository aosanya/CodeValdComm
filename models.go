// Package codevaldcomm provides real-time communication management for CodeVald
// agencies — channels, messages, threads, reactions, read receipts, and
// participants — backed by entitygraph.DataManager and entitygraph.SchemaManager.
package codevaldcomm

import (
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// CommDataManager is a type alias for entitygraph.DataManager scoped to the
// comm domain. All entity and relationship operations go through this interface.
type CommDataManager = entitygraph.DataManager

// CommSchemaManager is a type alias for entitygraph.SchemaManager scoped to the
// comm domain. Schema seeding and retrieval go through this interface.
type CommSchemaManager = entitygraph.SchemaManager

// CrossPublisher is implemented by internal/registrar. HTTP handlers call
// Publish to fire comm lifecycle events (message sent, edited, thread promoted,
// reaction added, member joined, participant presence).
type CrossPublisher = eventbus.Publisher
