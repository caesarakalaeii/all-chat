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

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

// StringCipher interface for encryption/decryption
type StringCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// UserRepository handles user data persistence
type UserRepository struct {
	db     *pgxpool.Pool
	cipher StringCipher
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool, cipher StringCipher) *UserRepository {
	return &UserRepository{db: db, cipher: cipher}
}

// DB returns the underlying connection pool for cross-table queries
// (e.g. DSGVO data export).
func (r *UserRepository) DB() *pgxpool.Pool {
	return r.db
}

// Create inserts a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	accessToken, err := r.encryptToken(user.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	refreshToken, err := r.encryptToken(user.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// granted_scopes is NOT NULL DEFAULT '{}'; passing an explicit value bypasses
	// the DEFAULT, so coalesce nil to an empty (non-nil) slice to avoid a NULL.
	grantedScopes := user.GrantedScopes
	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	query := `
INSERT INTO users (
id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
is_admin, access_token, refresh_token, token_expires_at, created_at, updated_at, granted_scopes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
`

	_, err = r.db.Exec(ctx, query,
		user.ID, user.TwitchID, user.GoogleID, user.KickID, user.AuthProvider, user.Username, user.DisplayName,
		user.ProfileImageURL, user.IsAdmin, accessToken, refreshToken,
		user.TokenExpiresAt, user.CreatedAt, user.UpdatedAt, grantedScopes,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByTwitchID retrieves a user by Twitch ID
func (r *UserRepository) GetByTwitchID(ctx context.Context, twitchID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_premium, is_beta_tester, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at, onboarding_completed_at
FROM users
WHERE twitch_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, twitchID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by Twitch ID: %w", err)
	}

	return user, nil
}

// GetByGoogleID retrieves a user by Google ID
func (r *UserRepository) GetByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_premium, is_beta_tester, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at, onboarding_completed_at
FROM users
WHERE google_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, googleID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by Google ID: %w", err)
	}

	return user, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	// Defensive: a nil pool is a misconfiguration; return a clear (non-not-found)
	// error rather than panicking deep in pgx. Callers that distinguish
	// transient-vs-terminal (e.g. HandleRefresh) then treat it as retryable
	// instead of force-logging-out the user.
	if r.db == nil {
		return nil, fmt.Errorf("user repository: nil database pool")
	}
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_premium, is_beta_tester, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at, onboarding_completed_at
FROM users
WHERE id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// GetByUsername retrieves a user by username (case-insensitive)
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_premium, is_beta_tester, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at, onboarding_completed_at
FROM users
WHERE LOWER(username) = LOWER($1)
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, username))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return user, nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()
	accessToken, err := r.encryptToken(user.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	refreshToken, err := r.encryptToken(user.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
UPDATE users
SET username = $2, display_name = $3, profile_image_url = $4,
	access_token = $5, refresh_token = $6, token_expires_at = $7, updated_at = $8
WHERE id = $1
`

	result, err := r.db.Exec(ctx, query,
		user.ID, user.Username, user.DisplayName, user.ProfileImageURL,
		accessToken, refreshToken, user.TokenExpiresAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// Delete removes a user by ID
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("user id is required")
	}

	result, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UpdateTokens updates only the OAuth tokens for a user
func (r *UserRepository) UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error {
	encAccessToken, err := r.encryptToken(accessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	encRefreshToken, err := r.encryptToken(refreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}
	query := `
UPDATE users
SET access_token = $2, refresh_token = $3, token_expires_at = $4, updated_at = $5
WHERE id = $1
`

	result, err := r.db.Exec(ctx, query, userID, encAccessToken, encRefreshToken, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UpdateGrantedScopes overwrites the OAuth scopes granted at the most recent consent.
//
// Kept separate from Update so that general user updates (which load the user via
// scanUser, where granted_scopes is intentionally not populated) can never clobber
// the stored scope set. Only the OAuth callback paths, which have the fresh token in
// hand, call this. nil is coalesced to an empty slice so the NOT NULL column is honoured.
func (r *UserRepository) UpdateGrantedScopes(ctx context.Context, userID string, scopes []string) error {
	if scopes == nil {
		scopes = []string{}
	}
	result, err := r.db.Exec(ctx,
		`UPDATE users SET granted_scopes = $2, updated_at = NOW() WHERE id = $1`,
		userID, scopes,
	)
	if err != nil {
		return fmt.Errorf("failed to update granted scopes: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// GetGrantedScopes returns the OAuth scopes granted at the most recent consent for a user.
// Used to decide whether a Twitch channel is eligible for EventSub chat reading without
// threading the column through the shared scanUser read paths.
func (r *UserRepository) GetGrantedScopes(ctx context.Context, userID string) ([]string, error) {
	var scopes []string
	err := r.db.QueryRow(ctx, `SELECT granted_scopes FROM users WHERE id = $1`, userID).Scan(&scopes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get granted scopes: %w", err)
	}
	return scopes, nil
}

// GetByKickID retrieves a user by Kick ID
func (r *UserRepository) GetByKickID(ctx context.Context, kickID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_premium, is_beta_tester, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at, onboarding_completed_at
FROM users
WHERE kick_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, kickID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by Kick ID: %w", err)
	}

	return user, nil
}

// StoreYouTubeToken stores a YouTube OAuth token in youtube_oauth_tokens keyed by
// channel ID. grantedScopes records the scopes granted at this consent so the
// moderation service (ADR-0017) can tell whether the channel has the opt-in
// youtube.force-ssl grant. On conflict, granted_scopes is MERGED (union) rather than
// replaced: a plain add-source (login scopes only) must never silently drop a
// previously granted force-ssl moderation scope. The access/refresh tokens are always
// replaced with the freshest values.
func (r *UserRepository) StoreYouTubeToken(ctx context.Context, userID, channelID string, token *oauth2.Token, grantedScopes []string) error {
	if channelID == "" {
		return fmt.Errorf("channel_id is required for storing YouTube tokens")
	}

	// Validate token is not already expired (with 5 minute buffer for clock skew)
	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("refusing to store expired YouTube token (expiry: %s, expired %.0f days ago)",
			token.Expiry.Format(time.RFC3339),
			time.Since(token.Expiry).Hours()/24,
		)
	}

	query := `
		INSERT INTO youtube_oauth_tokens (
			user_id, channel_id, access_token, refresh_token, token_type, expiry, encryption_version, granted_scopes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			encryption_version = EXCLUDED.encryption_version,
			granted_scopes = ARRAY(SELECT DISTINCT unnest(youtube_oauth_tokens.granted_scopes || EXCLUDED.granted_scopes)),
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	accessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	refreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		userID, channelID, accessToken, refreshToken, tokenType,
		token.Expiry, 1, grantedScopes, now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to store YouTube token: %w", err)
	}

	return nil
}

// StoreTwitchToken persists Twitch credentials obtained via the add-source
// link flow into twitch_oauth_tokens, keyed by Twitch login (ADR-0016). This
// is how a non-Twitch-login account (YouTube/Kick signup) gets its channel's
// EventSub chat grant into a place the partition predicate can see.
func (r *UserRepository) StoreTwitchToken(ctx context.Context, userID, twitchUserID, twitchLogin string, token *oauth2.Token, grantedScopes []string) error {
	if twitchLogin == "" {
		return fmt.Errorf("twitch_login is required for storing linked Twitch tokens")
	}

	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("refusing to store expired Twitch token (expiry: %s)", token.Expiry.Format(time.RFC3339))
	}

	accessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	refreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	query := `
		INSERT INTO twitch_oauth_tokens (
			user_id, twitch_user_id, twitch_login, access_token, refresh_token,
			token_expires_at, granted_scopes, encryption_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 1, NOW(), NOW())
		ON CONFLICT (user_id, twitch_login)
		DO UPDATE SET
			twitch_user_id = EXCLUDED.twitch_user_id,
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_expires_at = EXCLUDED.token_expires_at,
			granted_scopes = EXCLUDED.granted_scopes,
			encryption_version = EXCLUDED.encryption_version,
			updated_at = NOW()
	`

	if _, err := r.db.Exec(ctx, query,
		userID, twitchUserID, twitchLogin, accessToken, refreshToken,
		token.Expiry, grantedScopes,
	); err != nil {
		return fmt.Errorf("failed to store linked Twitch token: %w", err)
	}

	return nil
}

// StoreKickToken persists Kick credentials obtained via the opt-in moderation
// re-consent flow into kick_oauth_tokens, keyed by the channel slug (ADR-0017). This is
// how a non-Kick-login account (Twitch/YouTube signup) that linked Kick gets a
// moderation credential into a place the moderation service can resolve: the row carries
// the NUMERIC broadcaster id (kick_user_id) the Kick moderation API keys on, and the
// granted_scopes (moderation:ban). On conflict, granted_scopes is MERGED (union) so a
// later listener-token write or narrower consent never drops the moderation grant; the
// access/refresh tokens and kick_user_id are replaced with the freshest values.
func (r *UserRepository) StoreKickToken(ctx context.Context, userID, channelSlug, kickUserID string, token *oauth2.Token, grantedScopes []string) error {
	if channelSlug == "" {
		return fmt.Errorf("channel_slug is required for storing linked Kick tokens")
	}

	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("refusing to store expired Kick token (expiry: %s)", token.Expiry.Format(time.RFC3339))
	}

	accessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	refreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}
	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	query := `
		INSERT INTO kick_oauth_tokens (
			user_id, channel_id, kick_user_id, access_token, refresh_token,
			token_type, expiry, granted_scopes, encryption_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, 'Bearer', $6, $7, 1, NOW(), NOW())
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			kick_user_id = EXCLUDED.kick_user_id,
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			granted_scopes = ARRAY(SELECT DISTINCT unnest(kick_oauth_tokens.granted_scopes || EXCLUDED.granted_scopes)),
			encryption_version = EXCLUDED.encryption_version,
			updated_at = NOW()
	`

	if _, err := r.db.Exec(ctx, query,
		userID, channelSlug, kickUserID, accessToken, refreshToken, token.Expiry, grantedScopes,
	); err != nil {
		return fmt.Errorf("failed to store linked Kick token: %w", err)
	}

	return nil
}

// GetPlatformGrantedScopes returns the OAuth scopes the user currently holds FOR a
// specific platform, reading the authoritative source for that platform: the users row
// when the platform is the user's login provider, otherwise the per-link token table
// (twitch_oauth_tokens / kick_oauth_tokens / youtube_oauth_tokens). The opt-in
// moderation re-consent flow unions these with the requested moderation scopes so the
// consent URL asks for a SUPERSET — without injecting an UNRELATED platform's scopes
// (reading users.granted_scopes for a linked account would put, e.g., a YouTube login's
// scopes into a Twitch consent URL, which the provider rejects). Returns an empty slice
// (never an error) when the user holds no credential for the platform yet.
func (r *UserRepository) GetPlatformGrantedScopes(ctx context.Context, userID, platform string) ([]string, error) {
	var authProvider string
	if err := r.db.QueryRow(ctx, `SELECT auth_provider FROM users WHERE id = $1`, userID).Scan(&authProvider); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to look up auth provider: %w", err)
	}
	if authProvider == platform {
		return r.GetGrantedScopes(ctx, userID)
	}

	// Linked account: the platform's grant lives in its per-link table. The table name
	// comes from a fixed allow-list (never user input), so the interpolation is safe.
	var table string
	switch platform {
	case "twitch":
		table = "twitch_oauth_tokens"
	case "kick":
		table = "kick_oauth_tokens"
	case "youtube":
		table = "youtube_oauth_tokens"
	default:
		return []string{}, nil
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT granted_scopes FROM %s WHERE user_id = $1`, table), userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s granted scopes: %w", table, err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	out := []string{}
	for rows.Next() {
		var scopes []string
		if err := rows.Scan(&scopes); err != nil {
			return nil, fmt.Errorf("failed to scan %s granted scopes: %w", table, err)
		}
		for _, s := range scopes {
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate %s granted scopes: %w", table, err)
	}
	return out, nil
}

// SetOnboardingCompleted marks the first-run setup guide as finished/dismissed
// (completed=true → NOW()) or re-arms it (completed=false → NULL, used by the
// "restart onboarding" action in Settings). Returns the new column value.
func (r *UserRepository) SetOnboardingCompleted(ctx context.Context, userID string, completed bool) (*time.Time, error) {
	var completedAt *time.Time
	err := r.db.QueryRow(ctx, `
		UPDATE users
		SET onboarding_completed_at = CASE WHEN $2 THEN NOW() ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING onboarding_completed_at
	`, userID, completed).Scan(&completedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to set onboarding completion: %w", err)
	}
	return completedAt, nil
}

func (r *UserRepository) scanUser(row pgx.Row) (*models.User, error) {
	user := &models.User{}
	var encryptedAccessToken, encryptedRefreshToken string
	// profile_image_url is a nullable TEXT column; scan into a pointer so a
	// NULL (e.g. accounts created without an avatar) does not fail the scan.
	var profileImageURL *string

	err := row.Scan(
		&user.ID, &user.TwitchID, &user.GoogleID, &user.KickID, &user.AuthProvider, &user.Username, &user.DisplayName,
		&profileImageURL, &user.IsAdmin, &user.IsPremium, &user.IsBetaTester, &user.IsBanned, &user.BannedAt, &user.BannedReason, &user.BannedBy,
		&encryptedAccessToken, &encryptedRefreshToken,
		&user.TokenExpiresAt, &user.CreatedAt, &user.UpdatedAt, &user.OnboardingCompletedAt,
	)
	if err != nil {
		return nil, err
	}
	if profileImageURL != nil {
		user.ProfileImageURL = *profileImageURL
	}

	user.AccessToken, err = r.decryptToken(encryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	user.RefreshToken, err = r.decryptToken(encryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	return user, nil
}

func (r *UserRepository) encryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Encrypt(token)
}

func (r *UserRepository) decryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Decrypt(token)
}

// GetAllUsers retrieves all users (admin only).
//
// This is a metadata-only listing: it deliberately does NOT select or decrypt
// the OAuth access/refresh tokens. The admin user list never uses them, and
// coupling the listing to token decryption meant a single account with a
// corrupt or legacy-plaintext token broke the entire endpoint with a 500.
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	query := `
SELECT u.id, u.twitch_id, u.google_id, u.kick_id, u.auth_provider, u.username, u.display_name, u.profile_image_url,
       u.is_admin, u.is_premium, u.is_beta_tester, u.premium_admin_override_expires_at,
       (u.is_banned OR bpi.platform_id IS NOT NULL) AS is_banned,
       COALESCE(u.banned_at, bpi.banned_at) AS banned_at,
       COALESCE(u.banned_reason, bpi.reason) AS banned_reason,
       COALESCE(u.banned_by, bpi.banned_by) AS banned_by,
       u.created_at, u.updated_at
FROM users u
LEFT JOIN LATERAL (
  SELECT bpi.platform_id, bpi.banned_at, bpi.reason, bpi.banned_by
  FROM banned_platform_ids bpi
  WHERE bpi.is_active = true AND (
    (bpi.platform = 'twitch' AND bpi.platform_id = u.twitch_id) OR
    (bpi.platform = 'youtube' AND bpi.platform_id = u.google_id) OR
    (bpi.platform = 'kick' AND bpi.platform_id = u.kick_id)
  )
  LIMIT 1
) bpi ON true
ORDER BY u.created_at DESC
`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		// profile_image_url is a nullable TEXT column; scan into a pointer so a
		// NULL (e.g. accounts created without an avatar) does not fail the scan.
		var profileImageURL *string

		err := rows.Scan(
			&user.ID, &user.TwitchID, &user.GoogleID, &user.KickID, &user.AuthProvider,
			&user.Username, &user.DisplayName, &profileImageURL,
			&user.IsAdmin, &user.IsPremium, &user.IsBetaTester, &user.PremiumExpiresAt,
			&user.IsBanned, &user.BannedAt, &user.BannedReason, &user.BannedBy,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if profileImageURL != nil {
			user.ProfileImageURL = *profileImageURL
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// GetUserByID retrieves a user by their ID (admin only)
func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `
SELECT u.id, u.twitch_id, u.google_id, u.kick_id, u.auth_provider, u.username, u.display_name, u.profile_image_url,
       u.is_admin, u.is_premium, u.is_beta_tester,
       (u.is_banned OR bpi.platform_id IS NOT NULL) AS is_banned,
       COALESCE(u.banned_at, bpi.banned_at) AS banned_at,
       COALESCE(u.banned_reason, bpi.reason) AS banned_reason,
       COALESCE(u.banned_by, bpi.banned_by) AS banned_by,
       u.access_token, u.refresh_token, u.token_expires_at, u.created_at, u.updated_at, u.onboarding_completed_at
FROM users u
LEFT JOIN LATERAL (
  SELECT bpi.platform_id, bpi.banned_at, bpi.reason, bpi.banned_by
  FROM banned_platform_ids bpi
  WHERE bpi.is_active = true AND (
    (bpi.platform = 'twitch' AND bpi.platform_id = u.twitch_id) OR
    (bpi.platform = 'youtube' AND bpi.platform_id = u.google_id) OR
    (bpi.platform = 'kick' AND bpi.platform_id = u.kick_id)
  )
  LIMIT 1
) bpi ON true
WHERE u.id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// BanUser bans a user account and deactivates their overlays/sources
func (r *UserRepository) BanUser(ctx context.Context, userID, adminID, reason string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update user ban status
	_, err = tx.Exec(ctx, `
		UPDATE users
		SET is_banned = true,
			banned_at = NOW(),
			banned_reason = $1,
			banned_by = $2
		WHERE id = $3
	`, reason, adminID, userID)
	if err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}

	// Deactivate all overlays for this user
	_, err = tx.Exec(ctx, `
		UPDATE overlays
		SET is_active = false
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate overlays: %w", err)
	}

	// Deactivate all sources for this user's overlays
	_, err = tx.Exec(ctx, `
		UPDATE overlay_chat_sources
		SET is_active = false
		WHERE overlay_id IN (SELECT id FROM overlays WHERE user_id = $1)
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate sources: %w", err)
	}

	return tx.Commit(ctx)
}

// UnbanUser removes ban from user account (overlays remain inactive - manual reactivation)
func (r *UserRepository) UnbanUser(ctx context.Context, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE users
		SET is_banned = false,
			banned_at = NULL,
			banned_reason = NULL,
			banned_by = NULL
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to unban user: %w", err)
	}

	// Also deactivate any active platform-level bans for this user's platform IDs
	_, err = tx.Exec(ctx, `
		UPDATE banned_platform_ids bpi
		SET is_active = false, unbanned_at = NOW()
		FROM users u
		WHERE u.id = $1
		  AND bpi.is_active = true
		  AND (
		    (bpi.platform = 'twitch'   AND bpi.platform_id = u.twitch_id) OR
		    (bpi.platform = 'youtube'  AND bpi.platform_id = u.google_id) OR
		    (bpi.platform = 'kick'     AND bpi.platform_id = u.kick_id)
		  )
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate platform bans: %w", err)
	}

	return tx.Commit(ctx)
}

// GetBannedUsers returns list of banned users with pagination
func (r *UserRepository) GetBannedUsers(ctx context.Context, limit, offset int) ([]*models.User, error) {
	query := `
		SELECT id, username, display_name, is_banned, banned_at, banned_reason, banned_by
		FROM users
		WHERE is_banned = true
		ORDER BY banned_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query banned users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.IsBanned, &u.BannedAt, &u.BannedReason, &u.BannedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan banned user: %w", err)
		}
		users = append(users, &u)
	}
	return users, nil
}

// BanPlatformID adds platform ID to ban list
func (r *UserRepository) BanPlatformID(ctx context.Context, platform, platformID, adminID, reason string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO banned_platform_ids (platform, platform_id, banned_by, reason)
		VALUES ($1, $2, $3, $4)
	`, platform, platformID, adminID, reason)
	if err != nil {
		return fmt.Errorf("failed to ban platform ID: %w", err)
	}
	return nil
}

// FindExistingUserBySource checks if any existing user has an overlay chat source
// with the given platform and channel_id. Returns the owning user's username if
// found, empty string otherwise. This is used during registration to detect
// duplicate accounts (same streamer registering via different platforms).
// Shared overlay sources (platform='shared_overlay') are excluded.
func (r *UserRepository) FindExistingUserBySource(ctx context.Context, platform, channelID string) (string, error) {
	query := `
		SELECT u.username
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		JOIN users u ON o.user_id = u.id
		WHERE ocs.platform = $1
		  AND LOWER(ocs.channel_id) = LOWER($2)
		  AND ocs.platform != 'shared_overlay'
		LIMIT 1
	`

	var username string
	err := r.db.QueryRow(ctx, query, platform, channelID).Scan(&username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to check for existing user by source: %w", err)
	}

	return username, nil
}

// CountByAuthProvider returns the number of users per auth_provider.
// The result is a map from provider name (e.g. "twitch", "youtube", "kick") to count.
func (r *UserRepository) CountByAuthProvider(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT auth_provider, COUNT(*) FROM users GROUP BY auth_provider`)
	if err != nil {
		return nil, fmt.Errorf("failed to count users by auth_provider: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var provider string
		var count int64
		if err := rows.Scan(&provider, &count); err != nil {
			return nil, fmt.Errorf("failed to scan user count row: %w", err)
		}
		counts[provider] = count
	}
	return counts, rows.Err()
}

// IsPlatformIDBanned checks if a platform ID is banned
func (r *UserRepository) IsPlatformIDBanned(ctx context.Context, platform, platformID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM banned_platform_ids
			WHERE platform = $1 AND platform_id = $2 AND is_active = true
		)
	`, platform, platformID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check platform ban: %w", err)
	}
	return exists, nil
}
