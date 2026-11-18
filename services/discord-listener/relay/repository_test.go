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

package relay

import (
	"context"
	"errors"
	"testing"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// The relay webhook URL is a credential: anyone holding it can post into the channel. These
// tests pin that it is encrypted at rest (ADR-0091), that rows written before encryption
// still relay, and that they are encrypted on first read.

const testWebhookURL = "https://discord.com/api/webhooks/1234567890/SECRET-webhook-token"

func testCipher(t *testing.T) *encryption.MultiKeyEncryptor {
	t.Helper()
	aes, err := encryption.NewAESEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	enc, err := encryption.NewMultiKeyEncryptor([]encryption.KeyEntry{{Kid: 0x01, Cipher: aes}}, nil)
	require.NoError(t, err)
	return enc
}

type execCall struct {
	sql  string
	args []any
}

// fakeDB records statements and serves fixed rows (pgxmock does not build against pgx v5.11).
type fakeDB struct {
	rows  [][]any
	execs []execCall
}

func (f *fakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &fakeRows{rows: f.rows}, nil
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execs = append(f.execs, execCall{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

type fakeRows struct {
	rows [][]any
	i    int
}

func (r *fakeRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *fakeRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	if len(row) != len(dest) {
		return errors.New("scan: column count mismatch")
	}
	for i, d := range dest {
		*(d.(*string)) = row[i].(string)
	}
	return nil
}
func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return r.rows[r.i-1], nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map                         { return nil }

func TestStoreWebhookURL_StoresCiphertextNotTheURL(t *testing.T) {
	cipher := testCipher(t)
	db := &fakeDB{}
	repo := NewRepository(db, cipher, zap.NewNop())

	require.NoError(t, repo.StoreWebhookURL(context.Background(), "source-1", testWebhookURL))

	require.NotEmpty(t, db.execs)
	write := db.execs[0]
	require.Len(t, write.args, 2)
	assert.Equal(t, "source-1", write.args[0])
	stored, ok := write.args[1].(string)
	require.True(t, ok)
	assert.NotContains(t, stored, "SECRET-webhook-token", "the webhook token must never reach the database in the clear")
	assert.NotContains(t, stored, "discord.com")

	plain, err := cipher.DecryptString(stored)
	require.NoError(t, err)
	assert.Equal(t, testWebhookURL, plain)
}

func TestGetRelayConfigs_DecryptsStoredURL(t *testing.T) {
	cipher := testCipher(t)
	sealed, err := cipher.EncryptString(testWebhookURL)
	require.NoError(t, err)
	db := &fakeDB{rows: [][]any{{"source-1", "overlay-1", sealed}}}

	configs, err := NewRepository(db, cipher, zap.NewNop()).GetRelayConfigs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []relayConfig{{OverlayID: "overlay-1", WebhookURL: testWebhookURL}}, configs)
	assert.Empty(t, db.execs, "an encrypted row needs no rewrite")
}

func TestGetRelayConfigs_LegacyPlaintextRelaysAndIsEncryptedInPlace(t *testing.T) {
	cipher := testCipher(t)
	db := &fakeDB{rows: [][]any{{"source-1", "overlay-1", testWebhookURL}}}

	configs, err := NewRepository(db, cipher, zap.NewNop()).GetRelayConfigs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []relayConfig{{OverlayID: "overlay-1", WebhookURL: testWebhookURL}}, configs,
		"a row written before encryption must keep relaying")

	require.Len(t, db.execs, 1, "the legacy row must be rewritten encrypted")
	rewrite := db.execs[0]
	assert.Contains(t, rewrite.sql, "relay_webhook_url")
	assert.Contains(t, rewrite.sql, "= $3", "the rewrite must only replace the value it read, never a newer one")
	require.Len(t, rewrite.args, 3)
	assert.Equal(t, "source-1", rewrite.args[0])
	stored := rewrite.args[1].(string)
	assert.NotContains(t, stored, "SECRET-webhook-token")
	plain, err := cipher.DecryptString(stored)
	require.NoError(t, err)
	assert.Equal(t, testWebhookURL, plain)
	assert.Equal(t, testWebhookURL, rewrite.args[2], "the guard compares against the plaintext that was read")
}

// The key-rotator does not sweep this JSONB key, so the listener moves it to the current key
// itself; otherwise retiring an old key would silently break the relay.
func TestGetRelayConfigs_ReencryptsURLsUnderAnOldKey(t *testing.T) {
	oldAES, err := encryption.NewAESEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	newAES, err := encryption.NewAESEncryptor([]byte("fedcba9876543210fedcba9876543210"))
	require.NoError(t, err)
	before, err := encryption.NewMultiKeyEncryptor([]encryption.KeyEntry{{Kid: 0x01, Cipher: oldAES}}, nil)
	require.NoError(t, err)
	after, err := encryption.NewMultiKeyEncryptor([]encryption.KeyEntry{{Kid: 0x01, Cipher: oldAES}, {Kid: 0x02, Cipher: newAES}}, nil)
	require.NoError(t, err)
	sealedV1, err := before.EncryptString(testWebhookURL)
	require.NoError(t, err)
	db := &fakeDB{rows: [][]any{{"source-1", "overlay-1", sealedV1}}}

	configs, err := NewRepository(db, after, zap.NewNop()).GetRelayConfigs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []relayConfig{{OverlayID: "overlay-1", WebhookURL: testWebhookURL}}, configs)
	require.Len(t, db.execs, 1)
	assert.Equal(t, sealedV1, db.execs[0].args[2], "the guard compares against the stored value that was read")
	plain, kid, err := after.DecryptStringWithKid(db.execs[0].args[1].(string))
	require.NoError(t, err)
	assert.Equal(t, testWebhookURL, plain)
	assert.Equal(t, after.CurrentKid(), kid)
}

func TestGetRelayConfigs_LegacyPlaintextWithoutKeyStillRelays(t *testing.T) {
	db := &fakeDB{rows: [][]any{{"source-1", "overlay-1", testWebhookURL}}}

	configs, err := NewRepository(db, nil, zap.NewNop()).GetRelayConfigs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []relayConfig{{OverlayID: "overlay-1", WebhookURL: testWebhookURL}}, configs)
	assert.Empty(t, db.execs, "without a key there is nothing to re-encrypt with")
}

// One row that cannot be read must not stop the relays of every other overlay.
func TestGetRelayConfigs_SkipsUndecryptableRows(t *testing.T) {
	cipher := testCipher(t)
	sealed, err := cipher.EncryptString(testWebhookURL)
	require.NoError(t, err)
	db := &fakeDB{rows: [][]any{
		{"source-1", "overlay-1", "bm90LWEtdmFsaWQtY2lwaGVydGV4dA=="},
		{"source-2", "overlay-2", sealed},
	}}

	configs, err := NewRepository(db, cipher, zap.NewNop()).GetRelayConfigs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []relayConfig{{OverlayID: "overlay-2", WebhookURL: testWebhookURL}}, configs)
}

// The config JSONB was once writable by clients, so a stored value is only ever posted to when
// it is a Discord webhook: anything else would make the listener POST chat to an arbitrary URL.
func TestGetRelayConfigs_SkipsURLsThatAreNotDiscordWebhooks(t *testing.T) {
	cipher := testCipher(t)
	sealedInternal, err := cipher.EncryptString("http://overlay-manager:8080/internal")
	require.NoError(t, err)
	db := &fakeDB{rows: [][]any{
		{"source-1", "overlay-1", "http://169.254.169.254/latest/meta-data"},
		{"source-2", "overlay-2", sealedInternal},
		{"source-3", "overlay-3", "https://discord.com.evil.example/api/webhooks/1/x"},
	}}

	configs, err := NewRepository(db, cipher, zap.NewNop()).GetRelayConfigs(context.Background())

	require.NoError(t, err)
	assert.Empty(t, configs)
	assert.Empty(t, db.execs, "a rejected value must not be re-encrypted either")
}
