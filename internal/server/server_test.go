package server_test

import (
	"context"
	"net"
	"testing"

	pb "github.com/aosanya/CodeValdComm/gen/go/codevaldcomm/v1"
	"github.com/aosanya/CodeValdComm/internal/server"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1 << 20

// fakeSchemaManager is a configurable stub for entitygraph.SchemaManager.
type fakeSchemaManager struct {
	getActive func(ctx context.Context, agencyID string) (types.Schema, error)
}

func (f *fakeSchemaManager) SetSchema(ctx context.Context, schema types.Schema) error { return nil }
func (f *fakeSchemaManager) GetSchema(ctx context.Context, agencyID string) (types.Schema, error) {
	return types.Schema{}, nil
}
func (f *fakeSchemaManager) Publish(ctx context.Context, agencyID string) error { return nil }
func (f *fakeSchemaManager) Activate(ctx context.Context, agencyID string, version int) error {
	return nil
}
func (f *fakeSchemaManager) GetActive(ctx context.Context, agencyID string) (types.Schema, error) {
	if f.getActive != nil {
		return f.getActive(ctx, agencyID)
	}
	return types.Schema{ID: "comm-schema-v1", Version: 1, Tag: "v1", AgencyID: agencyID}, nil
}
func (f *fakeSchemaManager) GetVersion(ctx context.Context, agencyID string, version int) (types.Schema, error) {
	return types.Schema{}, nil
}
func (f *fakeSchemaManager) ListVersions(ctx context.Context, agencyID string) ([]types.Schema, error) {
	return nil, nil
}

// newTestClient spins up an in-memory gRPC server backed by the given SchemaManager
// and returns a connected CommServiceClient.
func newTestClient(t *testing.T, sm entitygraph.SchemaManager) pb.CommServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterCommServiceServer(srv, server.New(sm))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewCommServiceClient(conn)
}

func TestGetSchema_Success(t *testing.T) {
	sm := &fakeSchemaManager{}
	client := newTestClient(t, sm)

	resp, err := client.GetSchema(context.Background(), &pb.GetSchemaRequest{AgencyId: "agency-1"})
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if resp.Id != "comm-schema-v1" {
		t.Errorf("ID = %q, want %q", resp.Id, "comm-schema-v1")
	}
	if resp.Version != 1 {
		t.Errorf("Version = %d, want 1", resp.Version)
	}
	if resp.AgencyId != "agency-1" {
		t.Errorf("AgencyId = %q, want %q", resp.AgencyId, "agency-1")
	}
}

func TestGetSchema_NotFound(t *testing.T) {
	sm := &fakeSchemaManager{
		getActive: func(_ context.Context, _ string) (types.Schema, error) {
			return types.Schema{}, entitygraph.ErrSchemaNotFound
		},
	}
	client := newTestClient(t, sm)

	_, err := client.GetSchema(context.Background(), &pb.GetSchemaRequest{AgencyId: "unknown"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.NotFound {
		t.Errorf("status code = %v, want %v", code, codes.NotFound)
	}
}
