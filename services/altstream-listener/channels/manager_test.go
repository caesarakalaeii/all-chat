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
	"fmt"
	"sort"
	"testing"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRepository_GetActiveChannels(t *testing.T) {
	db := &mockQueryExecutor{rows: [][]any{
		{"source-1", "overlay-1", "CopyGirl", true},
		{"source-2", "overlay-1", "new_channel", false}, // newly added: demand decides, not is_active
		{"source-3", "overlay-2", "https://altstream.online/x", true},
	}}
	repo := &Repository{db: db, logger: zap.NewNop()}

	chs, err := repo.GetActiveChannels(context.Background())
	require.NoError(t, err)
	require.Len(t, chs, 2, "a value that is not an Altstream username never reaches the wire")
	assert.Equal(t, "copygirl", chs[0].ChannelName, "usernames are case-insensitive and keyed lowercased")
	assert.Equal(t, "new_channel", chs[1].ChannelName)
	assert.False(t, chs[1].IsActive)
	assert.Contains(t, db.lastQuery, "ocs.platform = 'altstream'")
}

// fakeClient records the stream lifecycle the manager drives.
type fakeClient struct{ open map[string]bool }

func (f *fakeClient) Connect(ch string) error    { f.open[ch] = true; return nil }
func (f *fakeClient) Disconnect(ch string)       { delete(f.open, ch) }
func (f *fakeClient) IsConnected(ch string) bool { return f.open[ch] }

func TestManager_OneStreamPerChannelFilteredByDemand(t *testing.T) {
	db := &mockQueryExecutor{rows: [][]any{
		{"s1", "overlay-1", "copygirl", true},
		{"s2", "overlay-2", "CopyGirl", true},
		{"s3", "overlay-3", "achie7240", true},
	}}
	client := &fakeClient{open: map[string]bool{}}
	m := NewManager(&Repository{db: db, logger: zap.NewNop()}, client, nil, nil, zap.NewNop())

	require.NoError(t, m.syncChannels())
	assert.Equal(t, map[string]bool{"copygirl": true, "achie7240": true}, client.open)
	overlays := m.OverlayIDsForChannel("copygirl")
	sort.Strings(overlays)
	assert.Equal(t, []string{"overlay-1", "overlay-2"}, overlays)

	// Only achie7240 keeps a watching overlay: copygirl's stream closes.
	m.UpdateDemandedSourceIDs(map[string]listener.DemandedSource{"s3": {SourceID: "s3"}})
	assert.Equal(t, map[string]bool{"achie7240": true}, client.open)
	assert.Empty(t, m.OverlayIDsForChannel("copygirl"))
	assert.Equal(t, 1, m.GetFilteredAssignmentCount())
	assert.Equal(t, []string{"achie7240"}, m.GetActiveChannels())
}

// mockQueryExecutor answers every Query with a fresh cursor over rows.
type mockQueryExecutor struct {
	rows      [][]any
	lastQuery string
}

func (m *mockQueryExecutor) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	m.lastQuery = sql
	return &fakeRows{data: m.rows}, nil
}

func (m *mockQueryExecutor) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

// fakeRows implements the pgx.Rows methods the repository calls; the
// embedded nil interface panics if anything else is used.
type fakeRows struct {
	pgx.Rows
	data  [][]any
	index int
}

func (r *fakeRows) Close()     {}
func (r *fakeRows) Err() error { return nil }

func (r *fakeRows) Next() bool {
	if r.index >= len(r.data) {
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.data[r.index-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan mismatch: dest=%d row=%d", len(dest), len(row))
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		default:
			return fmt.Errorf("unsupported destination type %T", dest[i])
		}
	}
	return nil
}
