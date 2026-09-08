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

package tokens

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FacebookSource resolves the acting human's own Page credential from
// facebook_oauth_tokens (migration 094). The row stores the non-expiring page
// token the auth-service captured at consent (ADR-0060): no refresh token, no
// expiry, so unlike Twitch/Kick/YouTube there is no Refresh — an expired or
// revoked token surfaces as a Graph 401/403 at call time and the streamer
// re-consents. channelID from the overlay is the Page id.
type FacebookSource struct {
	db     *pgxpool.Pool
	cipher Cipher
}

// FacebookCredential is a decrypted, ready-to-use Page credential.
type FacebookCredential struct {
	AccessToken   string
	PageID        string
	GrantedScopes []string
}

// NewFacebookSource builds a FacebookSource.
func NewFacebookSource(db *pgxpool.Pool, cipher Cipher) *FacebookSource {
	return &FacebookSource{db: db, cipher: cipher}
}

// facebookResolveQuery selects the requesting user's own page credential for
// the channel (Page id). facebook sources exist only through the OAuth link
// flow, so there is no users-row fallback: no row means no credential.
const facebookResolveQuery = `
	SELECT access_token, page_id, granted_scopes
	FROM facebook_oauth_tokens
	WHERE user_id = $1 AND page_id = $2
	LIMIT 1`

// Resolve returns the requesting user's decrypted page credential for
// channelID (a Page id). Returns ErrNoCredential when the user holds none.
func (s *FacebookSource) Resolve(ctx context.Context, userID, channelID string) (*FacebookCredential, error) {
	var (
		encToken string
		pageID   string
		scopes   []string
	)
	err := s.db.QueryRow(ctx, facebookResolveQuery, userID, channelID).Scan(&encToken, &pageID, &scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCredential
	}
	if err != nil {
		return nil, fmt.Errorf("resolve facebook credential: %w", err)
	}

	token, err := s.cipher.DecryptString(encToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt facebook page token: %w", err)
	}
	return &FacebookCredential{AccessToken: token, PageID: pageID, GrantedScopes: scopes}, nil
}

// OwnerFacebookAnchor proves the overlay owner controls the Page. The
// credential row is created by Facebook Login itself, so the row's existence
// is the ownership evidence (same argument as the YouTube anchor: it exists
// because the provider issued a token for that Page's own account). It
// returns no id — Facebook moderation is addressed by the Page id the source
// already carries — so it is a pure gate.
func (s *FacebookSource) OwnerFacebookAnchor(ctx context.Context, ownerUserID, channelID string) error {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM facebook_oauth_tokens WHERE user_id = $1 AND page_id = $2)`,
		ownerUserID, channelID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("resolve owner facebook anchor: %w", err)
	}
	if !exists {
		return ErrOwnerChannelUnverified
	}
	return nil
}

// Compile-time: *FacebookSource satisfies the credential resolver shape.
var _ = func(s *FacebookSource) error { _, _ = s.Resolve(nil, "", ""); return nil }
