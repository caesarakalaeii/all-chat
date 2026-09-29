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

// Package memory is the bot's persistent learning store (the bot_memories table),
// ported from the former TypeScript MemoryRepository. It retrieves tag-relevant
// memories ranked by a staleness score, deduplicates on write by (type + >=2 shared
// tags), and prunes the stalest entry past a cap.
package memory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Type is the kind of memory recorded.
type Type string

const (
	TypeErrorPattern    Type = "error_pattern"
	TypeCorrection      Type = "correction"
	TypeCodebaseInsight Type = "codebase_insight"
)

// ValidType reports whether t is a recognized memory type.
func ValidType(t Type) bool {
	switch t {
	case TypeErrorPattern, TypeCorrection, TypeCodebaseInsight:
		return true
	}
	return false
}

// Stored is a memory row.
type Stored struct {
	ID          int
	Type        Type
	Tags        []string
	Content     string
	AccessCount int
	UpdatedAt   time.Time
}

// Marker is a memory to store.
type Marker struct {
	Type    Type
	Tags    []string
	Content string
}

const maxMemories = 500
const maxContentLen = 500

// staleness ranks memories: older + less-accessed = staler (sorted ASC to surface the
// freshest/most-used first).
const staleness = `EXTRACT(epoch FROM NOW() - updated_at) / 86400.0 - (access_count * 2.0)`

// Repository provides access to bot_memories.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Retrieve returns up to 10 memories whose tags overlap the query tags, freshest
// first, and bumps their access counts. Returns an empty slice when tags is empty.
func (r *Repository) Retrieve(ctx context.Context, tags []string) ([]Stored, error) {
	tags = NormalizeTags(tags)
	if len(tags) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, type, tags, content, access_count, updated_at
		   FROM bot_memories
		  WHERE tags && $1
		  ORDER BY `+staleness+` ASC
		  LIMIT 10`, tags)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Stored
	var ids []int
	for rows.Next() {
		var m Stored
		if err := rows.Scan(&m.ID, &m.Type, &m.Tags, &m.Content, &m.AccessCount, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		_, _ = r.pool.Exec(ctx,
			`UPDATE bot_memories SET access_count = access_count + 1 WHERE id = ANY($1)`, ids)
	}
	return out, nil
}

// Store inserts a new memory or, if an existing memory of the same type shares at
// least two tags, updates that one instead. Prunes the stalest entry past the cap.
func (r *Repository) Store(ctx context.Context, m Marker) error {
	tags := NormalizeTags(m.Tags)
	content := truncate(m.Content, maxContentLen)

	var existingID int
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM bot_memories
		  WHERE type = $1
		    AND cardinality(ARRAY(SELECT unnest(tags) INTERSECT SELECT unnest($2::text[]))) >= 2
		  LIMIT 1`, string(m.Type), tags).Scan(&existingID)
	switch {
	case err == nil:
		if _, err := r.pool.Exec(ctx,
			`UPDATE bot_memories SET content = $1, tags = $2, updated_at = NOW() WHERE id = $3`,
			content, tags, existingID); err != nil {
			return err
		}
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO bot_memories (type, tags, content) VALUES ($1, $2, $3)`,
			string(m.Type), tags, content); err != nil {
			return err
		}
	default:
		return err
	}
	return r.pruneIfNeeded(ctx)
}

// Update replaces the content of an existing memory by id.
func (r *Repository) Update(ctx context.Context, id int, content string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE bot_memories SET content = $1, updated_at = NOW() WHERE id = $2`,
		truncate(content, maxContentLen), id)
	return err
}

func (r *Repository) pruneIfNeeded(ctx context.Context) error {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM bot_memories`).Scan(&count); err != nil {
		return err
	}
	if count > maxMemories {
		_, err := r.pool.Exec(ctx,
			`DELETE FROM bot_memories WHERE id = (
				SELECT id FROM bot_memories ORDER BY `+staleness+` DESC LIMIT 1
			)`)
		return err
	}
	return nil
}

// NormalizeTags lowercases, trims, and drops empty tags.
func NormalizeTags(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var (
	knownServices = []string{
		"twitch-listener", "youtube-listener", "youtube-innertube-listener", "kick-listener",
		"tiktok-listener", "discord-listener", "api-gateway", "auth-service", "emote-service",
		"message-processor", "overlay-manager", "source-manager", "token-refresh-service", "support-bot",
	}
	knownKeywords = []string{
		"oomkill", "crashloop", "timeout", "quota", "rate-limit", "429", "500", "502", "503",
		"connection", "websocket", "redis", "postgres", "database",
	}
)

// ExtractTags derives memory-relevant tags from a question (service names + error
// keywords it mentions).
func ExtractTags(question string) []string {
	lower := strings.ToLower(question)
	var found []string
	for _, s := range knownServices {
		if strings.Contains(lower, s) {
			found = append(found, s)
		}
	}
	for _, k := range knownKeywords {
		if strings.Contains(lower, k) {
			found = append(found, k)
		}
	}
	return found
}
