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

package discord

import (
	"sync"
	"testing"
	"time"
)

func TestSerialQueuesFIFO(t *testing.T) {
	q := newSerialQueues()
	const n = 50
	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		q.enqueue("chan-1", func() {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			wg.Done()
		})
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if order[i] != i {
			t.Fatalf("tasks ran out of order at %d: %v", i, order[:min(i+3, n)])
		}
	}
}

func TestSerialQueuesCleansUp(t *testing.T) {
	q := newSerialQueues()
	var wg sync.WaitGroup
	wg.Add(1)
	q.enqueue("ephemeral", func() { wg.Done() })
	wg.Wait()

	// After the drain finishes it must delete the key so the map does not grow
	// unboundedly over the thread-ID key space.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		q.mu.Lock()
		empty := len(q.pending) == 0 && len(q.running) == 0
		q.mu.Unlock()
		if empty {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("serialQueues did not clean up idle keys")
}

func TestChunkMessage(t *testing.T) {
	if got := chunkMessage(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("empty -> %v", got)
	}
	long := make([]rune, 5000)
	for i := range long {
		long[i] = 'a'
	}
	chunks := chunkMessage(string(long))
	if len(chunks) < 3 {
		t.Fatalf("expected >=3 chunks for 5000 runes, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len([]rune(c)) > discordMaxMessage {
			t.Fatalf("chunk %d exceeds %d runes", i, discordMaxMessage)
		}
	}
}
