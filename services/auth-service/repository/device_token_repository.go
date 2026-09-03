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

// Persistence for paired-device tokens and pending device-link requests
// (device_tokens + device_link_requests, migration 088; ADR-0049 steps 2-3).
//
// The rules from api_token_repository.go carry over unchanged, plus two more that are
// specific to a linking flow:
//
//   - No plaintext, ever. Every function takes a digest the caller computed
//     (middleware.HashDeviceToken / sha256 of the user code); nothing here can return,
//     log or store a secret.
//   - No projection selects token_hash, user_code_hash or auth_code_hash. A digest that
//     never leaves this file cannot be serialised into a response by accident.
//   - State transitions are single SQL statements with the guard in the WHERE clause,
//     not read-then-write. "Consume this code if it is still unconsumed" has to be one
//     atomic step or a replay wins the race.
//   - The attempts counter is incremented IN SQL. That counter is the brute-force bound
//     for the typed pairing code (ADR-0049 flags it as the security-review item), and a
//     count read in Go and written back would let two concurrent guesses both see the
//     same value.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxDeviceTokensPerUser caps how many live paired devices one account may hold. The
// cap exists for the same reason api_tokens has one: a plugin that re-links on every
// launch would otherwise accumulate credentials nobody revokes. It is generous — a
// streamer with several machines and several overlays is normal.
const MaxDeviceTokensPerUser = 20

// MaxLinkAttempts is the number of failed pairing-code attempts a single link request
// tolerates before the row is dead. THIS is the brute-force bound for the fallback
// flow: 8 characters from a 32-symbol alphabet with 5 guesses per 10-minute request is
// not a searchable space, and the gateway rate limit in front of it is defence in
// depth rather than the bound.
const MaxLinkAttempts = 5

// LinkRequestTTL is how long a pending link request lives. Ten minutes is long enough
// for a streamer to find the dashboard on a second machine and short enough that an
// abandoned code is not sitting there during the next stream.
const LinkRequestTTL = 10 * time.Minute

// AuthCodeTTL is the lifetime of the one-time authorization code minted at approval.
// Kept well under the request TTL: the plugin is polling and collects it within
// milliseconds of the redirect, so a five-minute ceiling is already generous.
const AuthCodeTTL = 5 * time.Minute

// Link flow discriminators, matching the CHECK constraint in migration 088.
const (
	// FlowLoopback is the primary path: the browser redirects to 127.0.0.1 and nothing
	// is typed.
	FlowLoopback = "loopback"
	// FlowCode is the fallback: the plugin shows an 8-character code the streamer types
	// into the dashboard, for when loopback cannot work (a second machine, a headless
	// host, a plugin that cannot bind a port).
	FlowCode = "code"
)

var (
	// ErrDeviceTokenLimitReached is returned when the user already holds
	// MaxDeviceTokensPerUser live devices.
	ErrDeviceTokenLimitReached = errors.New("paired device limit reached")
	// ErrLinkRequestNotFound covers unknown, expired, already-consumed and
	// attempt-exhausted requests. Deliberately one error: the four cases must be
	// indistinguishable to a client or the response becomes a probing oracle.
	ErrLinkRequestNotFound = errors.New("device link request not found")
	// ErrLinkRequestPending is returned by the exchange when the streamer has not
	// approved yet. This one IS distinguishable, because the plugin needs it: it is the
	// 428 it polls against.
	ErrLinkRequestPending = errors.New("device link request is pending approval")
	// ErrLinkRequestReplayed is returned when a code is presented a second time. The
	// caller must revoke the token the first exchange minted: a replay means the code
	// leaked, so the credential it produced is no longer trustworthy.
	ErrLinkRequestReplayed = errors.New("device link code has already been used")
)

// DeviceToken is the METADATA of a paired device — without the digest, and obviously
// without the plaintext. This is exactly what GET /me/devices may expose.
//
// There is no field for a plaintext token and there never will be: unlike a PAT, whose
// secret is shown to the user once, a device token's secret goes to the PLUGIN over the
// loopback redirect and is never rendered in a browser at all.
type DeviceToken struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	OverlayID   string     `json:"overlay_id"`
	OverlayName string     `json:"overlay_name"`
	Scopes      []string   `json:"scopes"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

// LinkRequest is the server-side view of a pending link. Every secret-bearing column is
// absent: the caller compares digests it computed itself.
type LinkRequest struct {
	ID              string
	Flow            string
	DeviceName      string
	RequestedScopes []string
	RedirectURI     string
	PKCEChallenge   string
	PKCEMethod      string
	UserID          string
	OverlayID       string
	GrantedScopes   []string
	Attempts        int
	ExpiresAt       time.Time
	ApprovedAt      *time.Time
	ConsumedAt      *time.Time
	// MintedTokenID is device_link_requests.device_token_id: the device this request
	// produced, if any. Populated only on the replay path, where the caller needs to
	// know what to revoke because a replayed code means the code leaked.
	MintedTokenID string
}

// DeviceTokenRepository owns device_tokens and device_link_requests.
type DeviceTokenRepository struct {
	db *pgxpool.Pool
}

// NewDeviceTokenRepository creates a DeviceTokenRepository.
func NewDeviceTokenRepository(db *pgxpool.Pool) *DeviceTokenRepository {
	return &DeviceTokenRepository{db: db}
}

// UserOwnsOverlay reports whether overlayID belongs to userID.
//
// The approve handler needs exactly this one fact, and it needs it before a credential
// exists: an overlay id that is not the streamer's must never become a device binding.
// CreateDeviceToken checks it again inside its transaction, because that is the last
// point before the row is written and "the handler already checked" is not a property
// worth relying on for the one column the whole per-overlay binding rests on.
func (r *DeviceTokenRepository) UserOwnsOverlay(ctx context.Context, userID, overlayID string) (bool, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM overlays WHERE id = $1 AND user_id = $2`, overlayID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("UserOwnsOverlay: %w", err)
	}
	return true, nil
}

// deviceTokenColumns is the shared projection for every device read. token_hash is
// absent on purpose (see the file comment).
const deviceTokenColumns = `
	d.id::text, d.name, d.overlay_id::text, COALESCE(o.name, ''), d.scopes,
	d.created_at, d.last_used_at, d.expires_at, d.revoked_at`

func scanDeviceToken(row pgx.Row) (*DeviceToken, error) {
	var d DeviceToken
	err := row.Scan(&d.ID, &d.Name, &d.OverlayID, &d.OverlayName, &d.Scopes,
		&d.CreatedAt, &d.LastUsedAt, &d.ExpiresAt, &d.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// linkRequestColumns is the projection for a pending request. The three digest columns
// are absent; callers that need to check one pass their own digest into the WHERE
// clause instead of reading it out.
const linkRequestColumns = `
	id::text, flow, device_name, requested_scopes, COALESCE(redirect_uri, ''),
	pkce_challenge, pkce_method, COALESCE(user_id::text, ''), COALESCE(overlay_id::text, ''),
	granted_scopes, attempts, expires_at, approved_at, consumed_at`

func scanLinkRequest(row pgx.Row) (*LinkRequest, error) {
	var r LinkRequest
	err := row.Scan(&r.ID, &r.Flow, &r.DeviceName, &r.RequestedScopes, &r.RedirectURI,
		&r.PKCEChallenge, &r.PKCEMethod, &r.UserID, &r.OverlayID, &r.GrantedScopes,
		&r.Attempts, &r.ExpiresAt, &r.ApprovedAt, &r.ConsumedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateLinkRequest opens a pending link.
//
// userCodeHash is nil for the loopback flow (no code is shown) and the SHA-256 of the
// 8-character code for the fallback. redirectURI must ALREADY have passed
// handlers.ValidateLoopbackRedirect; this function does not re-validate it, because a
// repository is the wrong layer to own a security rule — the caller owns it and the
// column comment in migration 088 says so.
func (r *DeviceTokenRepository) CreateLinkRequest(
	ctx context.Context,
	flow string,
	userCodeHash []byte,
	pkceChallenge, pkceMethod, redirectURI, deviceName string,
	requestedScopes []string,
	ttl time.Duration,
) (*LinkRequest, error) {
	if flow != FlowLoopback && flow != FlowCode {
		return nil, fmt.Errorf("CreateLinkRequest: unknown flow %q", flow)
	}
	if requestedScopes == nil {
		requestedScopes = []string{}
	}
	var redirect *string
	if redirectURI != "" {
		redirect = &redirectURI
	}
	// The TTL is multiplied into an interval rather than concatenated into one.
	// `($8 || ' seconds')::INTERVAL` reads fine but types $8 as `text`, because `||`
	// only has a text overload — and pgx v5 refuses to encode an int64 into a text
	// parameter ("cannot find encode plan"), so EVERY call failed with a 500 and
	// device linking could not be started at all. Multiplication takes the numeric
	// straight through. Do not reintroduce the concatenation: it is not a style
	// preference, it is the difference between working and a hard failure, and
	// TestTTLIntervalsAreNotStringConcatenated pins it.
	row := r.db.QueryRow(ctx, `
		INSERT INTO device_link_requests
			(flow, user_code_hash, pkce_challenge, pkce_method, redirect_uri,
			 device_name, requested_scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + $8 * INTERVAL '1 second')
		RETURNING `+linkRequestColumns,
		flow, userCodeHash, pkceChallenge, pkceMethod, redirect,
		deviceName, requestedScopes, int64(ttl/time.Second))
	req, err := scanLinkRequest(row)
	if err != nil {
		return nil, fmt.Errorf("CreateLinkRequest: %w", err)
	}
	return req, nil
}

// GetPendingLinkRequest reads a live, unconsumed request by id.
//
// Expired, consumed and attempt-exhausted rows are reported as ErrLinkRequestNotFound,
// so the approve screen cannot be used to enumerate ids or learn why a request died.
func (r *DeviceTokenRepository) GetPendingLinkRequest(ctx context.Context, id string) (*LinkRequest, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+linkRequestColumns+`
		  FROM device_link_requests
		 WHERE id = $1
		   AND consumed_at IS NULL
		   AND expires_at > NOW()
		   AND attempts < $2`, id, MaxLinkAttempts)
	req, err := scanLinkRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetPendingLinkRequest: %w", err)
	}
	return req, nil
}

// FindPendingByUserCode resolves a typed pairing code to its request, counting a wrong
// code as a failed attempt.
//
// The attempt accounting is the whole point of this function and it is why the miss path
// is a WRITE. An UPDATE that increments every live code-flow row would be wrong (one
// wrong guess must not kill somebody else's pairing), so a miss cannot be attributed to
// a row — instead, the CALLER is rate-limited at the gateway, and the per-request
// counter below bounds guesses against a KNOWN request id, which is the case that
// matters: an attacker who has the id and is guessing the code.
//
// A hit does not touch attempts: a correct code is not an attempt against anything.
func (r *DeviceTokenRepository) FindPendingByUserCode(ctx context.Context, userCodeHash []byte) (*LinkRequest, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+linkRequestColumns+`
		  FROM device_link_requests
		 WHERE user_code_hash = $1
		   AND flow = $2
		   AND consumed_at IS NULL
		   AND expires_at > NOW()
		   AND attempts < $3`, userCodeHash, FlowCode, MaxLinkAttempts)
	req, err := scanLinkRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("FindPendingByUserCode: %w", err)
	}
	return req, nil
}

// registerFailedAttemptSQL is the brute-force bound, named so a test can assert that it
// is still checked and incremented in ONE statement rather than read-then-written.
const registerFailedAttemptSQL = `
	UPDATE device_link_requests
	   SET attempts = attempts + 1
	 WHERE id = $1 AND consumed_at IS NULL
	RETURNING attempts`

// RegisterFailedAttempt increments the per-request failure counter in SQL and reports
// whether the request is now dead.
//
// Checked and incremented in one statement so two concurrent guesses cannot both read
// the same count. The row is not deleted at the ceiling: leaving it with
// attempts >= MaxLinkAttempts means every subsequent lookup misses through the same
// predicate, so there is one rule for "dead" rather than two.
func (r *DeviceTokenRepository) RegisterFailedAttempt(ctx context.Context, id string) (dead bool, err error) {
	var attempts int
	err = r.db.QueryRow(ctx, registerFailedAttemptSQL, id).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("RegisterFailedAttempt: %w", err)
	}
	return attempts >= MaxLinkAttempts, nil
}

// ApproveLinkRequest binds a pending request to the approving user, an overlay and a
// granted scope set, and stores the digest of the one-time authorization code.
//
// The user_id predicate is absent on purpose: an unapproved request belongs to NOBODY
// (device_link_requests.user_id is NULL until this call), so ownership is established
// here rather than checked here. What guards the call is the session on the endpoint —
// only a signed-in streamer reaches it, and the row they approve becomes theirs.
//
// `AND user_id IS NULL` makes approval single-shot: a second approve of the same
// request finds no row, so two dashboard tabs cannot mint two tokens from one code.
func (r *DeviceTokenRepository) ApproveLinkRequest(
	ctx context.Context,
	id, userID, overlayID string,
	grantedScopes []string,
	deviceName string,
	authCodeHash []byte,
	authCodeTTL time.Duration,
) (*LinkRequest, error) {
	if grantedScopes == nil {
		grantedScopes = []string{}
	}
	row := r.db.QueryRow(ctx, `
		UPDATE device_link_requests
		   SET user_id = $2,
		       overlay_id = $3,
		       granted_scopes = $4,
		       device_name = COALESCE(NULLIF($5, ''), device_name),
		       approved_at = NOW(),
		       auth_code_hash = $6,
		       auth_code_expires_at = NOW() + $7 * INTERVAL '1 second'
		 WHERE id = $1
		   AND user_id IS NULL
		   AND consumed_at IS NULL
		   AND expires_at > NOW()
		   AND attempts < $8
		RETURNING `+linkRequestColumns,
		id, userID, overlayID, grantedScopes, deviceName, authCodeHash,
		int64(authCodeTTL/time.Second), MaxLinkAttempts)
	req, err := scanLinkRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ApproveLinkRequest: %w", err)
	}
	return req, nil
}

// DenyLinkRequest terminates a pending request without minting anything. Deny and
// Approve both end the row, so a streamer who says no leaves nothing behind for the
// plugin to keep polling against.
func (r *DeviceTokenRepository) DenyLinkRequest(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE device_link_requests
		   SET consumed_at = COALESCE(consumed_at, NOW())
		 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("DenyLinkRequest: %w", err)
	}
	return nil
}

// consumeAuthCodeSQL is the claim statement, named so a test can assert its guards
// without a database (see device_token_repository_test.go). Callers append
// linkRequestColumns.
const consumeAuthCodeSQL = `
	UPDATE device_link_requests
	   SET consumed_at = NOW(),
	       auth_code_hash = NULL
	 WHERE id = $1
	   AND consumed_at IS NULL
	   AND approved_at IS NOT NULL
	   AND (
	         (flow = $3 AND auth_code_hash = $2 AND auth_code_expires_at > NOW())
	      OR (flow = $4 AND user_code_hash = $2 AND expires_at > NOW())
	       )
	RETURNING `

// ConsumeAuthCode claims the one-time authorization code, atomically.
//
// This is the single most delicate statement in the feature, so its properties are
// spelled out:
//
//   - ONE statement. The guard (`auth_code_hash = $2`, unexpired, unconsumed) and the
//     effect (clear the hash, stamp consumed_at) happen together, so two concurrent
//     exchanges cannot both win.
//   - The code is INVALIDATED WHETHER OR NOT the exchange goes on to succeed. That is
//     what "one-time" has to mean: if the PKCE verifier turns out to be wrong, the code
//     is still burnt, because the presenter has already demonstrated they hold it.
//   - A replay is detectable afterwards. consumed_at and device_token_id survive, so the
//     caller can tell "already used" from "never existed" and revoke the token the
//     first exchange minted.
//
// EITHER DIGEST CLAIMS THE ROW, and that is what keeps the two delivery paths on one
// state machine rather than two:
//
//   - loopback: the plugin presents the one-time authorization code it received over
//     the redirect, so `claimHash` is its digest and matches auth_code_hash.
//   - code flow: there is no redirect and therefore no second secret to deliver. The
//     plugin presents the pairing code it displayed, whose digest is already in
//     user_code_hash, and the streamer's approval is what authorises the claim.
//
// Both are single-use for the same reason: consumed_at is stamped by this statement,
// and a consumed row is terminal whichever digest claimed it. The auth code is also
// cleared, so a loopback code cannot be replayed even against a row that was somehow
// approved twice.
//
// Returns the request when the claim succeeded. ErrLinkRequestReplayed (with the minted
// token id) when the row exists but was already consumed. ErrLinkRequestPending when the
// row exists and has not been approved. ErrLinkRequestNotFound otherwise.
func (r *DeviceTokenRepository) ConsumeAuthCode(ctx context.Context, id string, claimHash []byte) (*LinkRequest, error) {
	row := r.db.QueryRow(ctx, consumeAuthCodeSQL+linkRequestColumns,
		id, claimHash, FlowLoopback, FlowCode)
	req, err := scanLinkRequest(row)
	if err == nil {
		return req, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ConsumeAuthCode: %w", err)
	}

	// The claim failed. Work out why, so the plugin gets 428 while it should keep
	// polling and 400 when it should stop — and so a replay can be acted on.
	var (
		approvedAt *time.Time
		consumedAt *time.Time
		tokenID    *string
		expired    bool
	)
	err = r.db.QueryRow(ctx, `
		SELECT approved_at, consumed_at, device_token_id::text, (expires_at <= NOW())
		  FROM device_link_requests
		 WHERE id = $1`, id).Scan(&approvedAt, &consumedAt, &tokenID, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ConsumeAuthCode classify: %w", err)
	}
	switch {
	case consumedAt != nil:
		// Already used. The caller revokes MintedTokenID: a replay means the code leaked,
		// so the credential the first exchange produced is no longer trustworthy either.
		out := &LinkRequest{ID: id, ApprovedAt: approvedAt, ConsumedAt: consumedAt}
		if tokenID != nil {
			out.MintedTokenID = *tokenID
		}
		return out, ErrLinkRequestReplayed
	case expired:
		return nil, ErrLinkRequestNotFound
	case approvedAt == nil:
		// Still waiting for the streamer. This is the 428 the plugin polls against.
		return nil, ErrLinkRequestPending
	default:
		// Approved, unconsumed, unexpired — so the presented code was simply wrong.
		return nil, ErrLinkRequestNotFound
	}
}

// CreateDeviceToken persists a paired device and links it back to the request that
// minted it.
//
// tokenHash must be the SHA-256 digest of the plaintext (middleware.HashDeviceToken).
// The plaintext is the caller's to hand to the plugin once; this function has no way to
// see it.
//
// The live-device cap is enforced inside a transaction that locks the owning user row,
// so two concurrent links cannot both observe "19 devices" and both insert. The
// device_link_requests back-reference is written in the SAME transaction, so a token
// always has an audit trail to the approval that authorised it.
func (r *DeviceTokenRepository) CreateDeviceToken(
	ctx context.Context,
	userID, overlayID, name string,
	tokenHash []byte,
	scopes []string,
	lifetime time.Duration,
	linkRequestID string,
) (*DeviceToken, error) {
	if len(tokenHash) == 0 {
		return nil, errors.New("CreateDeviceToken: empty token hash")
	}
	if scopes == nil {
		scopes = []string{}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("CreateDeviceToken begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedUser string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&lockedUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("CreateDeviceToken lock user: %w", err)
	}

	// The overlay must belong to the approving user. Checked here as well as in the
	// handler because this is the last place before a credential exists: an overlay id
	// that is not theirs must not become a binding, whatever route reached us.
	var ownedOverlay string
	err = tx.QueryRow(ctx,
		`SELECT id::text FROM overlays WHERE id = $1 AND user_id = $2`, overlayID, userID).Scan(&ownedOverlay)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("CreateDeviceToken check overlay: %w", err)
	}

	var live int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM device_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()`, userID).Scan(&live); err != nil {
		return nil, fmt.Errorf("CreateDeviceToken count: %w", err)
	}
	if live >= MaxDeviceTokensPerUser {
		return nil, ErrDeviceTokenLimitReached
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO device_tokens (user_id, overlay_id, name, token_hash, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, NOW() + $6 * INTERVAL '1 second')
		RETURNING id::text`,
		userID, overlayID, name, tokenHash, scopes, int64(lifetime/time.Second)).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("CreateDeviceToken insert: %w", err)
	}

	if linkRequestID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE device_link_requests SET device_token_id = $2 WHERE id = $1`,
			linkRequestID, id); err != nil {
			return nil, fmt.Errorf("CreateDeviceToken link back-reference: %w", err)
		}
	}

	row := tx.QueryRow(ctx, `
		SELECT `+deviceTokenColumns+`
		  FROM device_tokens d
		  LEFT JOIN overlays o ON o.id = d.overlay_id
		 WHERE d.id = $1`, id)
	token, err := scanDeviceToken(row)
	if err != nil {
		return nil, fmt.Errorf("CreateDeviceToken read back: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("CreateDeviceToken commit: %w", err)
	}
	return token, nil
}

// ListDeviceTokensByUser returns the user's paired devices, newest first.
//
// Revoked and expired rows are included so the list can show what lapsed and when;
// they are self-evidently unusable from revoked_at / expires_at. Nothing in the
// returned struct can identify the secret.
func (r *DeviceTokenRepository) ListDeviceTokensByUser(ctx context.Context, userID string) ([]DeviceToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+deviceTokenColumns+`
		  FROM device_tokens d
		  LEFT JOIN overlays o ON o.id = d.overlay_id
		 WHERE d.user_id = $1
		 ORDER BY d.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListDeviceTokensByUser query: %w", err)
	}
	defer rows.Close()

	devices := []DeviceToken{}
	for rows.Next() {
		device, err := scanDeviceToken(rows)
		if err != nil {
			return nil, fmt.Errorf("ListDeviceTokensByUser scan: %w", err)
		}
		devices = append(devices, *device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListDeviceTokensByUser rows: %w", err)
	}
	return devices, nil
}

// RevokeDeviceToken marks one of the user's devices revoked and returns its metadata.
//
// The user_id predicate is the authorization check: a device id belonging to someone
// else is indistinguishable from one that does not exist (ErrNotFound), so this
// endpoint cannot be used to probe for other users' device ids.
//
// revoked_at is set only when it is still NULL, making a second revoke a no-op rather
// than a rewrite of history, while the row is still returned so the endpoint is
// idempotent from the client's perspective.
func (r *DeviceTokenRepository) RevokeDeviceToken(ctx context.Context, userID, deviceID string) (*DeviceToken, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		UPDATE device_tokens
		   SET revoked_at = COALESCE(revoked_at, NOW())
		 WHERE id = $1 AND user_id = $2
		RETURNING id::text`, deviceID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("RevokeDeviceToken: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		SELECT `+deviceTokenColumns+`
		  FROM device_tokens d
		  LEFT JOIN overlays o ON o.id = d.overlay_id
		 WHERE d.id = $1`, id)
	token, err := scanDeviceToken(row)
	if err != nil {
		return nil, fmt.Errorf("RevokeDeviceToken read back: %w", err)
	}
	return token, nil
}

// RevokeDeviceTokenByID revokes a device without an owner check, for the replay path.
//
// It is deliberately separate from RevokeDeviceToken so the owner-scoped call used by
// the endpoint cannot be bypassed by accident. The only caller is the exchange handler
// reacting to a replayed authorization code, where there is no session to attribute the
// revocation to and the token being killed was just minted from the leaked code.
func (r *DeviceTokenRepository) RevokeDeviceTokenByID(ctx context.Context, deviceID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE device_tokens
		   SET revoked_at = COALESCE(revoked_at, NOW())
		 WHERE id = $1`, deviceID)
	if err != nil {
		return fmt.Errorf("RevokeDeviceTokenByID: %w", err)
	}
	return nil
}

// SweepExpiredLinkRequests deletes pending rows that will never complete.
//
// Unlike device_tokens, an abandoned link request has no value at all once it expires —
// it holds no credential and nothing references it — so it is deleted rather than kept.
// Rows that DID mint a token are retained regardless of age, because they are the audit
// trail for that token.
func (r *DeviceTokenRepository) SweepExpiredLinkRequests(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM device_link_requests
		 WHERE device_token_id IS NULL
		   AND expires_at < NOW() - INTERVAL '1 hour'`)
	if err != nil {
		return 0, fmt.Errorf("SweepExpiredLinkRequests: %w", err)
	}
	return tag.RowsAffected(), nil
}
