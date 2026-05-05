// Package arangodb implements the ArangoDB backend for CodeValdComm.
// All implementation logic lives in
// [github.com/aosanya/CodeValdSharedLib/entitygraph/arangodb]; this package
// is a thin service-scoped adapter that fixes the collection and graph names
// to their CodeValdComm-specific values.
//
// Entity collections (document):
//   - comm_groups       — Channel entities
//   - comm_participants — Participant entities
//   - comm_messages     — Message entities
//   - comm_edit_history — EditHistory entities (immutable)
//   - comm_attachments  — Attachment entities (immutable)
//
// Infrastructure collections:
//   - comm_entities          — fallback document collection
//   - comm_relationships     — ArangoDB EDGE collection (must be created as edge)
//   - comm_schemas_draft     — mutable draft schema document per agency
//   - comm_schemas_published — immutable published schema snapshots
//
// Named graph: comm_graph
//
// Use [New] to obtain a (DataManager, SchemaManager) pair from an open database.
// Use [NewBackend] to connect and construct in a single call.
package arangodb

import (
	"fmt"

	driver "github.com/arangodb/go-driver"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	sharedadb "github.com/aosanya/CodeValdSharedLib/entitygraph/arangodb"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Backend is a type alias for the shared ArangoDB Backend.
type Backend = sharedadb.Backend

// Config is the connection parameters for the CodeValdComm ArangoDB backend.
// It is an alias of [sharedadb.ConnConfig].
type Config = sharedadb.ConnConfig

// toSharedConfig expands a CodeValdComm Config into a full SharedLib Config,
// filling in the fixed CodeValdComm-specific collection and graph names.
func toSharedConfig(cfg Config) sharedadb.Config {
	return sharedadb.Config{
		Endpoint:            cfg.Endpoint,
		Username:            cfg.Username,
		Password:            cfg.Password,
		Database:            cfg.Database,
		Schema:              cfg.Schema,
		EntityCollection:    "comm_entities",
		RelCollection:       "comm_relationships",
		SchemasDraftCol:     "comm_schemas_draft",
		SchemasPublishedCol: "comm_schemas_published",
		GraphName:           "comm_graph",
	}
}

// New constructs a Backend from an already-open driver.Database using the
// provided schema, ensures all collections and the named graph exist, and
// returns the Backend as both a DataManager and a SchemaManager.
func New(db driver.Database, schema types.Schema) (entitygraph.DataManager, entitygraph.SchemaManager, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("arangodb: New: database must not be nil")
	}
	scfg := toSharedConfig(Config{Schema: schema})
	return sharedadb.New(db, scfg)
}

// NewBackend connects to ArangoDB using cfg, ensures all collections exist
// (including comm_relationships as an edge collection), bootstraps the
// comm_graph named graph, and returns a ready-to-use Backend.
// cfg.Database is required (e.g. "codevaldcomm").
func NewBackend(cfg Config) (*Backend, error) {
	if cfg.Database == "" {
		return nil, fmt.Errorf("arangodb: NewBackend: Database must be set (e.g. \"codevaldcomm\")")
	}
	scfg := toSharedConfig(cfg)
	return sharedadb.NewBackend(scfg)
}

// NewBackendFromDB constructs a Backend from an already-open driver.Database.
// Intended for tests that manage their own database lifecycle.
func NewBackendFromDB(db driver.Database, schema types.Schema) (*Backend, error) {
	if db == nil {
		return nil, fmt.Errorf("arangodb: NewBackendFromDB: database must not be nil")
	}
	scfg := toSharedConfig(Config{Schema: schema})
	return sharedadb.NewBackendFromDB(db, scfg)
}
