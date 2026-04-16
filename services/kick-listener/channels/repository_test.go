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
	"database/sql"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

func TestRepository_GetActiveChannelsHandlesStringChatroomIDs(t *testing.T) {
	rows := newFakeRows([][]any{
		{"source-1", "overlay-1", "channel-one", "123", true},
		{"source-2", "overlay-2", "channel-two", "invalid", true},
	})

	mockDB := &mockQueryExecutor{rows: rows}
	repo := &Repository{db: mockDB, logger: zap.NewNop()}

	channels, err := repo.GetActiveChannels(context.Background())
	if err != nil {
		t.Fatalf("GetActiveChannels returned error: %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	if channels[0].ChatroomID != 123 {
		t.Fatalf("expected first chatroom ID to be 123, got %d", channels[0].ChatroomID)
	}

	if channels[1].ChatroomID != 0 {
		t.Fatalf("expected invalid chatroom ID to fall back to 0, got %d", channels[1].ChatroomID)
	}

	if !rows.closed {
		t.Fatalf("expected rows to be closed after iteration")
	}

	if mockDB.lastQuery == "" {
		t.Fatalf("expected query to be executed")
	}
}

// TestRepository_GetActiveChannelsIncludesInactiveSources verifies that newly
// added sources with is_active=false are returned (demand filtering handles eligibility).
func TestRepository_GetActiveChannelsIncludesInactiveSources(t *testing.T) {
	rows := newFakeRows([][]any{
		{"source-1", "overlay-1", "channel-active", "100", true},
		{"source-2", "overlay-1", "channel-new", "", false},
	})

	mockDB := &mockQueryExecutor{rows: rows}
	repo := &Repository{db: mockDB, logger: zap.NewNop()}

	channels, err := repo.GetActiveChannels(context.Background())
	if err != nil {
		t.Fatalf("GetActiveChannels returned error: %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels (including inactive source), got %d", len(channels))
	}

	if channels[0].IsActive != true {
		t.Fatalf("expected first channel to be active")
	}
	if channels[1].IsActive != false {
		t.Fatalf("expected second channel (newly added) to be inactive")
	}
	if channels[1].ChannelSlug != "channel-new" {
		t.Fatalf("expected second channel slug to be 'channel-new', got %s", channels[1].ChannelSlug)
	}
}

type mockQueryExecutor struct {
	rows      pgx.Rows
	lastQuery string
}

func (m *mockQueryExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.lastQuery = sql
	return m.rows, nil
}

func (m *mockQueryExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, fmt.Errorf("not implemented")
}

type fakeRows struct {
	data   [][]any
	index  int
	closed bool
	err    error
}

func newFakeRows(data [][]any) *fakeRows {
	return &fakeRows{data: data}
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.index >= len(r.data) {
		r.Close()
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.data) {
		return fmt.Errorf("no current row")
	}

	row := r.data[r.index-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan mismatch: dest=%d row=%d", len(dest), len(row))
	}

	for i := range dest {
		if err := assignScanValue(dest[i], row[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.data) {
		return nil, fmt.Errorf("no current row")
	}
	row := r.data[r.index-1]
	values := make([]any, len(row))
	copy(values, row)
	return values, nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

func assignScanValue(dest any, value any) error {
	switch d := dest.(type) {
	case *string:
		if value == nil {
			*d = ""
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		*d = str
		return nil
	case *bool:
		if value == nil {
			*d = false
			return nil
		}
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
		*d = b
		return nil
	case *sql.NullString:
		if value == nil {
			*d = sql.NullString{}
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		*d = sql.NullString{String: str, Valid: true}
		return nil
	default:
		return fmt.Errorf("unsupported destination type %T", dest)
	}
}
