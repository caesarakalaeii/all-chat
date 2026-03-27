package listener_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// startTestServer starts an HTTP server on a random port and returns it with its listener.
// Call srv.Shutdown() or ShutdownCoordinator to stop it.
func startTestServer(t *testing.T) *http.Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: http.NewServeMux()}
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			// expected on shutdown — not a test failure
		}
	}()
	return srv
}

func newTestLeadershipListener(t *testing.T) *listener.LeadershipListener {
	t.Helper()
	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		DisableDemandFiltering: true,
	}, nil, nil)
	require.NoError(t, err)
	return ll
}

func TestShutdownCoordinator_CallsStopAndDrainsServer(t *testing.T) {
	defer goleak.VerifyNone(t)
	ll := newTestLeadershipListener(t)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))
	cancel() // trigger ctx cancellation so goroutines begin draining

	srv := startTestServer(t)
	// ShutdownCoordinator must complete without panic and return before test ends
	done := make(chan struct{})
	go func() {
		listener.ShutdownCoordinator(ll, mgr, nil, srv, nil)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("ShutdownCoordinator did not complete within 5 seconds")
	}
}

func TestShutdownCoordinator_PlatformDisconnectCalled(t *testing.T) {
	defer goleak.VerifyNone(t)
	ll := newTestLeadershipListener(t)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))
	cancel()

	var disconnectCalled bool
	platformDisconnect := func() { disconnectCalled = true }

	srv := startTestServer(t)
	listener.ShutdownCoordinator(ll, mgr, platformDisconnect, srv, nil)

	assert.True(t, disconnectCalled, "platformDisconnect should have been called")
}

func TestShutdownCoordinator_NilPlatformDisconnect(t *testing.T) {
	defer goleak.VerifyNone(t)
	ll := newTestLeadershipListener(t)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))
	cancel()

	srv := startTestServer(t)
	// Must not panic when platformDisconnect is nil
	assert.NotPanics(t, func() {
		listener.ShutdownCoordinator(ll, mgr, nil, srv, nil)
	})
}
