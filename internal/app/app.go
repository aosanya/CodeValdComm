// Package app holds the shared runtime wiring for CodeValdComm. Both the
// production binary (cmd/server) and the local dev binary (cmd/dev) call
// Run; they differ only in which environment variables they set before
// loading config.
package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"

	codevaldcomm "github.com/aosanya/CodeValdComm"
	pb "github.com/aosanya/CodeValdComm/gen/go/codevaldcomm/v1"
	"github.com/aosanya/CodeValdComm/internal/config"
	"github.com/aosanya/CodeValdComm/internal/httphandler"
	"github.com/aosanya/CodeValdComm/internal/registrar"
	"github.com/aosanya/CodeValdComm/internal/server"
	commdb "github.com/aosanya/CodeValdComm/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	entitypb "github.com/aosanya/CodeValdSharedLib/gen/go/entitygraph/v1"
	healthpb "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldhealth/v1"
	"github.com/aosanya/CodeValdSharedLib/health"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Run starts all CodeValdComm subsystems (Cross registrar, ArangoDB backend,
// gRPC + HTTP via cmux) and blocks until SIGINT/SIGTERM triggers graceful shutdown.
func Run(cfg config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Signal handling ───────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("codevaldcomm: shutdown signal received")
		cancel()
	}()

	// ── ArangoDB backend ──────────────────────────────────────────────────────
	backend, err := commdb.NewBackend(commdb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
		Schema:   codevaldcomm.DefaultCommSchema(),
	})
	if err != nil {
		return fmt.Errorf("ArangoDB backend: %w", err)
	}

	// ── Schema seed (idempotent on startup) ───────────────────────────────────
	if cfg.AgencyID != "" {
		seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := entitygraph.SeedSchema(seedCtx, backend, cfg.AgencyID, codevaldcomm.DefaultCommSchema()); err != nil {
			log.Printf("codevaldcomm: schema seed: %v", err)
		}
		seedCancel()
	} else {
		log.Println("codevaldcomm: CODEVALDCOMM_AGENCY_ID not set — skipping startup schema seed")
	}

	// ── Cross registrar (optional) ────────────────────────────────────────────
	var pub codevaldcomm.CrossPublisher
	if cfg.CrossGRPCAddr != "" {
		reg, regErr := registrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			cfg.AgencyID,
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if regErr != nil {
			log.Printf("codevaldcomm: registrar: %v — continuing without registration", regErr)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
			pub = reg
		}
	} else {
		log.Println("codevaldcomm: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer, _ := serverutil.NewGRPCServer()
	entitypb.RegisterEntityServiceServer(grpcServer, server.NewEntityServer(backend))
	pb.RegisterCommServiceServer(grpcServer, server.New(backend))
	healthpb.RegisterHealthServiceServer(grpcServer, health.New("codevaldcomm"))

	// ── HTTP convenience handler (9 comm flows) ───────────────────────────────
	httpHandler := httphandler.New(backend, backend, pub)
	httpServer := &http.Server{
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// ── TCP listener + cmux ───────────────────────────────────────────────────
	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.ListenAddr, err)
	}

	mux := cmux.New(lis)
	grpcLis := mux.MatchWithWriters(
		cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
	)
	httpLis := mux.Match(cmux.Any())

	go func() {
		if err := grpcServer.Serve(grpcLis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("codevaldcomm: gRPC server error: %v", err)
		}
	}()
	go func() {
		if err := httpServer.Serve(httpLis); err != nil && err != http.ErrServerClosed {
			log.Printf("codevaldcomm: HTTP server error: %v", err)
		}
	}()
	go func() {
		if err := mux.Serve(); err != nil {
			log.Printf("codevaldcomm: cmux serve error: %v", err)
		}
	}()

	log.Printf("codevaldcomm: listening on %s (gRPC + HTTP via cmux)", cfg.ListenAddr)

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	log.Println("codevaldcomm: shutting down")

	grpcServer.GracefulStop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("codevaldcomm: HTTP shutdown error: %v", err)
	}

	log.Println("codevaldcomm: stopped")
	return nil
}
