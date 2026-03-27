package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/coordination"
	"github.com/redis/go-redis/v9"
)

// StartTestRedis starts an in-memory miniredis server for tests.
// The server is NOT automatically closed — caller must close it explicitly before goleak.VerifyNone.
// Returns the miniredis server and its address (host:port).
func StartTestRedis(t *testing.T) (*miniredis.Miniredis, string) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	return mr, mr.Addr()
}

// StartTestRedisWithClient starts miniredis and returns both the server and a connected go-redis client.
// The caller must explicitly close the client and server (in that order) before goleak.VerifyNone fires,
// to allow miniredis goroutines to drain cleanly.
func StartTestRedisWithClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, addr := StartTestRedis(t)
	rc := redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	if err := rc.Ping(context.Background()).Err(); err != nil {
		mr.Close()
		t.Fatalf("failed to connect to test Redis at %s: %v", addr, err)
	}
	return mr, rc
}

// NewTestRedisClient creates a go-redis client connected to the given address.
// The client is automatically closed when the test ends.
func NewTestRedisClient(t *testing.T, addr string) *redis.Client {
	t.Helper()
	rc := redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	if err := rc.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("failed to connect to test Redis at %s: %v", addr, err)
	}
	t.Cleanup(func() { rc.Close() })
	return rc
}

// DelayedMockCoordinator is a MockCoordinator that delays QueryAssignments
// for a configurable duration to simulate slow initial assignment loading.
type DelayedMockCoordinator struct {
	Delay       time.Duration
	Assignments []*coordination.Assignment
}

func (m *DelayedMockCoordinator) PublishHeartbeat(_ context.Context, _ string) error {
	return nil
}

func (m *DelayedMockCoordinator) QueryAssignments(ctx context.Context, _ string) ([]*coordination.Assignment, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.Delay):
	}
	if m.Assignments != nil {
		return m.Assignments, nil
	}
	return []*coordination.Assignment{}, nil
}

func (m *DelayedMockCoordinator) StartJWTRefresh(_ context.Context) {}
func (m *DelayedMockCoordinator) StopJWTRefresh()                    {}
