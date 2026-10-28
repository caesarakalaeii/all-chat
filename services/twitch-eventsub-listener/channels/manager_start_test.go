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

package channels

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// hungPostgres returns a pool whose server accepts TCP connections but never answers, so any
// query blocks until its context is cancelled. It stands in for an initial sync that takes
// longer than the liveness window, which is what a sequential resolve of hundreds of channels
// does in production.
func hungPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			// Swallow the startup message and never reply; returns once the client hangs up.
			go func() { _, _ = io.Copy(io.Discard, c) }()
		}
	}()
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@"+ln.Addr().String()+"/db?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_ = ln.Close()
	})
	return pool
}

// The HTTP server (and with it the liveness probe) only starts after Start returns, so Start must
// not wait for the initial sync. With the sync inline, a pod tracking enough channels never got
// past startup before the kubelet killed it (2026-10-28 crashloop).
func TestStart_DoesNotBlockOnInitialSync(t *testing.T) {
	m := NewManager(hungPostgres(t), zap.NewNop(), &countingResolver{}, nil, time.Minute)

	returned := make(chan struct{})
	go func() {
		_ = m.Start(context.Background())
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("Start blocked on the initial sync")
	}

	stopped := make(chan struct{})
	go func() {
		m.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not abort the in-flight initial sync")
	}
}
