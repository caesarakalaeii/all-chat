package listener

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ShutdownCoordinator performs ordered graceful shutdown in three phases:
//
//  1. Stop mgr and base concurrently — waits for both to complete.
//  2. Call platformDisconnect() if non-nil — disconnects the platform client.
//  3. Drain the HTTP server with a 10-second timeout.
func ShutdownCoordinator(base *ListenerBase, mgr ChannelManager, platformDisconnect func(), srv *http.Server, logger *zap.Logger) {
	// Phase 1: stop channel manager and base goroutines concurrently.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mgr.Stop()
	}()
	go func() {
		defer wg.Done()
		base.Stop()
	}()
	wg.Wait()

	// Phase 2: disconnect the platform client (e.g. IRC disconnect, WebSocket close).
	if platformDisconnect != nil {
		platformDisconnect()
	}

	// Phase 3: drain HTTP server with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}
}
