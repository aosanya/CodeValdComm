// Package registrar provides the CodeValdComm service registrar.
// It wraps the shared-library heartbeat registrar, subscribes to
// cross.agency.created for schema seeding, and implements
// [codevaldcomm.CrossPublisher] so HTTP handlers can notify CodeValdCross of
// comm lifecycle events.
package registrar

import (
	"context"
	"log"
	"time"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	"github.com/aosanya/CodeValdComm/internal/httphandler"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
	"github.com/aosanya/CodeValdSharedLib/schemaroutes"
	entityserver "github.com/aosanya/CodeValdSharedLib/entitygraph/server"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Registrar handles two responsibilities:
//  1. Sending periodic heartbeat registrations to CodeValdCross.
//  2. Implementing [codevaldcomm.CrossPublisher] so HTTP handlers can fire
//     comm lifecycle events (message sent, edited, thread promoted, …).
//
// Construct via [New]; start heartbeats by calling Run in a goroutine;
// stop by cancelling the context then calling Close.
type Registrar struct {
	heartbeat sharedregistrar.Registrar
}

// Compile-time assertion that *Registrar implements codevaldcomm.CrossPublisher.
var _ codevaldcomm.CrossPublisher = (*Registrar)(nil)

// New constructs a Registrar that heartbeats to the CodeValdCross gRPC server
// at crossAddr.
//
//   - crossAddr     — host:port of the CodeValdCross gRPC server
//   - advertiseAddr — host:port that Cross dials back on
//   - agencyID      — agency this instance serves (empty = all agencies)
//   - pingInterval  — heartbeat cadence; ≤ 0 means only the initial ping
//   - pingTimeout   — per-RPC timeout for each Register call
func New(
	crossAddr, advertiseAddr, agencyID string,
	pingInterval, pingTimeout time.Duration,
) (*Registrar, error) {
	hb, err := sharedregistrar.New(
		crossAddr,
		advertiseAddr,
		agencyID,
		"codevaldcomm",
		httphandler.AllTopics(),
		[]string{"cross.agency.created"},
		commRoutes(agencyID),
		pingInterval,
		pingTimeout,
	)
	if err != nil {
		return nil, err
	}
	return &Registrar{heartbeat: hb}, nil
}

// Run starts the heartbeat loop. Must be called inside a goroutine.
func (r *Registrar) Run(ctx context.Context) {
	r.heartbeat.Run(ctx)
}

// Close releases the underlying gRPC connection.
func (r *Registrar) Close() {
	r.heartbeat.Close()
}

// Publish implements [eventbus.Publisher].
// Best-effort notification — logs the event. A future iteration will call a
// CodeValdCross Publish RPC once it is available.
func (r *Registrar) Publish(_ context.Context, e eventbus.Event) error {
	log.Printf("registrar[codevaldcomm]: publish topic=%q agencyID=%q payload=%T",
		e.Topic, e.AgencyID, e.Payload)
	return nil
}

// commRoutes returns the HTTP routes CodeValdComm registers with Cross.
// Schema-derived CRUD routes (for Channel, Participant, Message, EditHistory,
// Attachment) point to EntityService. Custom comm flows are exposed separately.
func commRoutes(agencyID string) []types.RouteInfo {
	schema := codevaldcomm.DefaultCommSchema()
	routes := schemaroutes.RoutesFromSchema(
		schema,
		"/{agencyId}/comm",
		"agencyId",
		entityserver.GRPCServicePath,
	)
	for _, rt := range routes {
		log.Printf("[registrar] route: %s %s → %s", rt.Method, rt.Pattern, rt.GrpcMethod)
	}
	return routes
}
