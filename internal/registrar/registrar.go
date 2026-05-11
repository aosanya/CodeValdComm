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
// Convenience flow routes (empty GrpcMethod) are prepended so they win
// first-match over the schema-derived gRPC routes for overlapping patterns.
func commRoutes(agencyID string) []types.RouteInfo {
	var routes []types.RouteInfo
	routes = append(routes, convenienceRoutes()...)
	schema := codevaldcomm.DefaultCommSchema()
	routes = append(routes, schemaroutes.RoutesFromSchema(
		schema,
		"/{agencyId}/comm",
		"agencyId",
		entityserver.GRPCServicePath,
	)...)
	for _, rt := range routes {
		log.Printf("[registrar] route: %s %s → %s", rt.Method, rt.Pattern, rt.GrpcMethod)
	}
	return routes
}

// convenienceRoutes returns direct-HTTP routes for CodeValdComm's eleven
// multi-step flows. GrpcMethod is intentionally empty so Cross forwards the
// request verbatim to CodeValdComm's HTTP handler instead of calling a gRPC
// method. These are prepended before schema-derived routes so they win on
// first-match for overlapping patterns.
func convenienceRoutes() []types.RouteInfo {
	return []types.RouteInfo{
		{Method: "POST", Pattern: "/{agencyId}/comm/participants", Capability: "create_participant", IsWrite: true},
		{Method: "PUT", Pattern: "/{agencyId}/comm/participants/{participantId}", Capability: "update_presence", IsWrite: true},
		{Method: "POST", Pattern: "/{agencyId}/comm/channels", Capability: "create_channel", IsWrite: true},
		{Method: "POST", Pattern: "/{agencyId}/comm/channels/{channelId}/members", Capability: "join_channel", IsWrite: true},
		{Method: "POST", Pattern: "/{agencyId}/comm/channels/{channelId}/messages", Capability: "send_message", IsWrite: true},
		{Method: "GET", Pattern: "/{agencyId}/comm/channels/{channelId}/messages", Capability: "list_messages"},
		{Method: "PUT", Pattern: "/{agencyId}/comm/messages/{messageId}", Capability: "edit_message", IsWrite: true},
		{Method: "PUT", Pattern: "/{agencyId}/comm/messages/{messageId}/promote", Capability: "promote_to_thread", IsWrite: true},
		{Method: "POST", Pattern: "/{agencyId}/comm/messages/{messageId}/reactions", Capability: "add_reaction", IsWrite: true},
		{Method: "POST", Pattern: "/{agencyId}/comm/messages/{messageId}/read", Capability: "mark_read", IsWrite: true},
		{Method: "POST", Pattern: "/{agencyId}/comm/direct", Capability: "create_dm", IsWrite: true},
	}
}
