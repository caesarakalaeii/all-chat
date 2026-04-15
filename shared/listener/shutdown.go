// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
//  1. Stop mgr and listener concurrently — waits for both to complete.
//  2. Call platformDisconnect() if non-nil — disconnects the platform client.
//  3. Drain the HTTP server with a 10-second timeout.
func ShutdownCoordinator(listener interface{ Stop() }, mgr ChannelManager, platformDisconnect func(), srv *http.Server, logger *zap.Logger) {
	// Phase 1: stop channel manager and listener goroutines concurrently.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mgr.Stop()
	}()
	go func() {
		defer wg.Done()
		listener.Stop()
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
		if logger != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}
	}
}
