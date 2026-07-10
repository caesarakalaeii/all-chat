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

import "sync"

// serialQueues runs tasks one-at-a-time per key (per Discord channel) via a single
// drain goroutine per active key. Tasks are enqueued under a lock, so execution order
// matches enqueue order (FIFO) and only one task per key runs at a time. When a key's
// queue drains, its entry is removed so the map does not grow without bound over the
// unbounded space of thread IDs.
//
// Note: discordgo dispatches each gateway event in its own goroutine, so the order in
// which two near-simultaneous messages reach enqueue is itself best-effort; this gives
// mutual exclusion + FIFO among enqueued tasks, not a global receive-order guarantee.
type serialQueues struct {
	mu      sync.Mutex
	pending map[string][]func()
	running map[string]bool
}

func newSerialQueues() *serialQueues {
	return &serialQueues{
		pending: make(map[string][]func()),
		running: make(map[string]bool),
	}
}

// enqueue appends a task for key and ensures a drain goroutine is running. It returns
// immediately; the task runs asynchronously.
func (s *serialQueues) enqueue(key string, fn func()) {
	s.mu.Lock()
	s.pending[key] = append(s.pending[key], fn)
	if !s.running[key] {
		s.running[key] = true
		go s.drain(key)
	}
	s.mu.Unlock()
}

func (s *serialQueues) drain(key string) {
	for {
		s.mu.Lock()
		tasks := s.pending[key]
		if len(tasks) == 0 {
			delete(s.pending, key)
			delete(s.running, key)
			s.mu.Unlock()
			return
		}
		fn := tasks[0]
		s.pending[key] = tasks[1:]
		s.mu.Unlock()

		fn() // run outside the lock so other keys are not blocked
	}
}

// chunkMessage splits text into <=2000-rune Discord messages, preferring to break on
// newlines.
func chunkMessage(text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return []string{""}
	}
	var chunks []string
	for len(runes) > discordMaxMessage {
		cut := discordMaxMessage
		// try to break on the last newline within the window
		for i := discordMaxMessage - 1; i > discordMaxMessage/2; i-- {
			if runes[i] == '\n' {
				cut = i + 1
				break
			}
		}
		chunks = append(chunks, string(runes[:cut]))
		runes = runes[cut:]
	}
	chunks = append(chunks, string(runes))
	return chunks
}

// truncateRunes caps s to n runes.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
