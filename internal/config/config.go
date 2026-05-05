// Package config loads CodeValdComm runtime configuration from environment variables.
package config

import (
	"time"

	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Config holds all runtime configuration for the CodeValdComm service.
type Config struct {
	// ListenAddr is the host:port for the combined gRPC + HTTP listener.
	// Controlled by CODEVALDCOMM_GRPC_PORT (default "50057").
	ListenAddr string

	// ArangoEndpoint is the ArangoDB HTTP endpoint (default "http://localhost:8529").
	ArangoEndpoint string

	// ArangoUser is the ArangoDB username (default "root").
	ArangoUser string

	// ArangoPassword is the ArangoDB password.
	ArangoPassword string

	// ArangoDatabase is the ArangoDB database name (default "codevaldcomm").
	ArangoDatabase string

	// CrossGRPCAddr is the CodeValdCross gRPC address. Empty disables registration.
	CrossGRPCAddr string

	// AdvertiseAddr is the address CodeValdCross dials back on.
	// Defaults to ListenAddr when unset.
	AdvertiseAddr string

	// AgencyID is the agency this instance is scoped to.
	// Empty means this instance serves all agencies.
	AgencyID string

	// PingInterval is the heartbeat cadence sent to CodeValdCross (default 20s).
	PingInterval time.Duration

	// PingTimeout is the per-RPC timeout for each Register call (default 5s).
	PingTimeout time.Duration
}

// Load reads runtime configuration from environment variables, falling back to
// sensible defaults for any variable that is unset or empty.
//
// Environment variables:
//
//	CODEVALDCOMM_GRPC_PORT     - listen port (default "50057")
//	COMM_ARANGO_ENDPOINT       - ArangoDB endpoint (default "http://localhost:8529")
//	COMM_ARANGO_USER           - ArangoDB username (default "root")
//	COMM_ARANGO_PASSWORD       - ArangoDB password (default "")
//	COMM_ARANGO_DATABASE       - ArangoDB database name (default "codevaldcomm")
//	CROSS_GRPC_ADDR            - CodeValdCross gRPC address (default ""; disables registration)
//	COMM_GRPC_ADVERTISE_ADDR   - address Cross dials back on (default: ListenAddr)
//	CODEVALDCOMM_AGENCY_ID     - agency scope for this instance (default ""; all agencies)
//	CROSS_PING_INTERVAL        - heartbeat cadence, Go duration string (default "20s")
//	CROSS_PING_TIMEOUT         - per-RPC Register timeout, Go duration string (default "5s")
func Load() Config {
	port := serverutil.EnvOrDefault("CODEVALDCOMM_GRPC_PORT", "50057")
	listenAddr := ":" + port
	return Config{
		ListenAddr:     listenAddr,
		ArangoEndpoint: serverutil.EnvOrDefault("COMM_ARANGO_ENDPOINT", "http://localhost:8529"),
		ArangoUser:     serverutil.EnvOrDefault("COMM_ARANGO_USER", "root"),
		ArangoPassword: serverutil.EnvOrDefault("COMM_ARANGO_PASSWORD", ""),
		ArangoDatabase: serverutil.EnvOrDefault("COMM_ARANGO_DATABASE", "codevaldcomm"),
		CrossGRPCAddr:  serverutil.EnvOrDefault("CROSS_GRPC_ADDR", ""),
		AdvertiseAddr:  serverutil.EnvOrDefault("COMM_GRPC_ADVERTISE_ADDR", listenAddr),
		AgencyID:       serverutil.EnvOrDefault("CODEVALDCOMM_AGENCY_ID", ""),
		PingInterval:   serverutil.ParseDurationString("CROSS_PING_INTERVAL", 20*time.Second),
		PingTimeout:    serverutil.ParseDurationString("CROSS_PING_TIMEOUT", 5*time.Second),
	}
}
