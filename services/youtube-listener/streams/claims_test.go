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

package streams

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/caesar/all-chat/shared/youtubeclaim"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type fakeEligibility struct {
	mu       sync.Mutex
	rows     []*models.StreamSource
	calls    int
	gateFree []bool
}

func (f *fakeEligibility) GetEligibleSources(_ context.Context, gateFree bool) ([]*models.StreamSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.gateFree = append(f.gateFree, gateFree)
	return f.rows, nil
}

func (f *fakeEligibility) set(rows ...*models.StreamSource) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = rows
}

type fakeVerifier struct {
	mu        sync.Mutex
	verified  map[string]bool // userID + "|" + channelID
	calls     int
	forgotten []string
	onVerify  func() // runs after the verdict is read, standing in for a concurrent caller
}

func (f *fakeVerifier) Verified(_ context.Context, userID, channelID string) (bool, error) {
	f.mu.Lock()
	f.calls++
	verified := f.verified[userID+"|"+channelID]
	onVerify := f.onVerify
	f.mu.Unlock()
	if onVerify != nil {
		onVerify()
	}
	return verified, nil
}

func (f *fakeVerifier) Forget(userID, channelID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, userID+"|"+channelID)
}

// refusingLeadershipClient models another replica holding the global-sync lease.
type refusingLeadershipClient struct{}

func (refusingLeadershipClient) ClaimLeadership(context.Context, string, string, string) (bool, error) {
	return false, nil
}
func (refusingLeadershipClient) RenewLeadership(context.Context, string, string, string) (bool, error) {
	return false, nil
}
func (refusingLeadershipClient) ReleaseLeadership(context.Context, string, string, string) error {
	return nil
}
func (refusingLeadershipClient) RegisterPeer(context.Context, string, string) (int, error) {
	return 2, nil
}

func row(overlayID, channelID, ownerUserID string) *models.StreamSource {
	return &models.StreamSource{OverlayID: overlayID, ChannelID: channelID, OwnerUserID: ownerUserID}
}

func newClaimsManager(t *testing.T, elig *fakeEligibility, verifier *fakeVerifier) (*Manager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return &Manager{
		logger:             zap.NewNop(),
		eligibility:        elig,
		verifier:           verifier,
		claims:             youtubeclaim.NewClaimStore(rc),
		gateFree:           func() bool { return false },
		quotaState:         func() quota.QuotaState { return quota.QuotaStateHealthy },
		syncLeaderStreamID: "global-sync",
		claimed:            make(map[string]string),
		connectedOverlays: map[string]time.Time{
			"ov1": time.Now(), "ov-forged": time.Now(), "ov-forged2": time.Now(), "ov-real": time.Now(),
		},
	}, mr
}

// A claim makes innertube drop the channel for every overlay, so it is only taken while one of
// the opted-in owner's overlays is connected; otherwise a viewer of someone else's overlay on
// that channel would get no chat from either listener.
func TestRefreshClaims_SkipsDisconnectedOwner(t *testing.T) {
	t.Run("connect_then_disconnect", func(t *testing.T) {
		elig := &fakeEligibility{rows: []*models.StreamSource{row("ov-offline", "UCowned", "user-1")}}
		verifier := &fakeVerifier{verified: map[string]bool{"user-1|UCowned": true}}
		m, mr := newClaimsManager(t, elig, verifier)

		m.refreshClaims(context.Background())
		if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
			t.Fatal("channel claimed while no opted-in overlay is connected")
		}

		m.connectedOverlays["ov-offline"] = time.Now()
		m.refreshClaims(context.Background())
		if !mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
			t.Fatal("channel not claimed once the owner's overlay connected")
		}

		delete(m.connectedOverlays, "ov-offline")
		m.refreshClaims(context.Background())
		if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
			t.Fatal("claim kept after the owner's overlay disconnected")
		}
	})

	// handleOverlayDisconnected drops the overlay from connectedOverlays at once but keeps its
	// pollers through the debounce; releasing the claim then would hand a page refresh to innertube.
	t.Run("pending_disconnect_debounce", func(t *testing.T) {
		elig := &fakeEligibility{rows: []*models.StreamSource{row("ov-refreshing", "UCowned", "user-1")}}
		verifier := &fakeVerifier{verified: map[string]bool{"user-1|UCowned": true}}
		m, mr := newClaimsManager(t, elig, verifier)
		timer := time.AfterFunc(time.Hour, func() {})
		t.Cleanup(func() { timer.Stop() })
		m.disconnectDebounceTimers = map[string]*time.Timer{"ov-refreshing": timer}

		m.refreshClaims(context.Background())
		if !mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
			t.Fatal("overlay inside the disconnect debounce must still count as connected for claims")
		}
	})
}

func TestRefreshClaims_ClaimsVerifiedEligible(t *testing.T) {
	elig := &fakeEligibility{rows: []*models.StreamSource{row("ov1", "UCowned", "user-1")}}
	verifier := &fakeVerifier{verified: map[string]bool{"user-1|UCowned": true}}
	m, mr := newClaimsManager(t, elig, verifier)
	m.gateFree = func() bool { return true }

	m.refreshClaims(context.Background())

	got, err := mr.Get(youtubeclaim.ClaimKey("UCowned"))
	if err != nil {
		t.Fatalf("claim missing: %v", err)
	}
	if got != "user-1" {
		t.Fatalf("claim value = %q, want the verified owner user-1", got)
	}
	if ttl := mr.TTL(youtubeclaim.ClaimKey("UCowned")); ttl <= 0 || ttl > youtubeclaim.DefaultClaimTTL {
		t.Fatalf("claim TTL = %v, want (0, %v]", ttl, youtubeclaim.DefaultClaimTTL)
	}
	if owner, ok := m.claimedOwner("UCowned"); !ok || owner != "user-1" {
		t.Fatalf("claimedOwner = %q, %v; want user-1, true", owner, ok)
	}
	if len(elig.gateFree) != 1 || !elig.gateFree[0] {
		t.Fatalf("gateFree passed to eligibility = %v, want [true] (evaluated on every round)", elig.gateFree)
	}
}

func TestRefreshClaims_SkipsUnverified(t *testing.T) {
	elig := &fakeEligibility{rows: []*models.StreamSource{
		// A copied token row: eligible in SQL, but the token belongs to another channel.
		row("ov-forged", "UCvictim", "forger"),
		// Two eligible rows on one channel: the forged one sorts first, the real owner wins.
		row("ov-forged2", "UCshared", "forger"),
		row("ov-real", "UCshared", "real-owner"),
	}}
	verifier := &fakeVerifier{verified: map[string]bool{"real-owner|UCshared": true}}
	m, mr := newClaimsManager(t, elig, verifier)

	m.refreshClaims(context.Background())

	if mr.Exists(youtubeclaim.ClaimKey("UCvictim")) {
		t.Fatal("unverified owner must not claim the channel")
	}
	if _, ok := m.claimedOwner("UCvictim"); ok {
		t.Fatal("unverified channel must not be in the claimed set")
	}
	if got, _ := mr.Get(youtubeclaim.ClaimKey("UCshared")); got != "real-owner" {
		t.Fatalf("UCshared claim value = %q, want real-owner", got)
	}
	if owner, _ := m.claimedOwner("UCshared"); owner != "real-owner" {
		t.Fatalf("claimedOwner(UCshared) = %q, want real-owner", owner)
	}
}

func TestRefreshClaims_ReleasesDroppedChannel(t *testing.T) {
	t.Run("no_longer_eligible", func(t *testing.T) {
		elig := &fakeEligibility{rows: []*models.StreamSource{row("ov1", "UClapsing", "user-1")}}
		verifier := &fakeVerifier{verified: map[string]bool{"user-1|UClapsing": true}}
		m, mr := newClaimsManager(t, elig, verifier)

		m.refreshClaims(context.Background())
		if !mr.Exists(youtubeclaim.ClaimKey("UClapsing")) {
			t.Fatal("first round should claim the channel")
		}

		elig.set()
		m.refreshClaims(context.Background())
		if mr.Exists(youtubeclaim.ClaimKey("UClapsing")) {
			t.Fatal("a channel that dropped out of eligibility must be released at once, not left to the TTL")
		}
		if _, ok := m.claimedOwner("UClapsing"); ok {
			t.Fatal("released channel must leave the claimed set so syncStreams stops processing it")
		}
	})

	for _, state := range []quota.QuotaState{quota.QuotaStateCritical, quota.QuotaStateExhausted, quota.QuotaStateDepleted} {
		t.Run("quota_"+string(state), func(t *testing.T) {
			elig := &fakeEligibility{rows: []*models.StreamSource{row("ov1", "UCbusy", "user-1")}}
			verifier := &fakeVerifier{verified: map[string]bool{"user-1|UCbusy": true}}
			m, mr := newClaimsManager(t, elig, verifier)

			m.refreshClaims(context.Background())
			if !mr.Exists(youtubeclaim.ClaimKey("UCbusy")) {
				t.Fatal("first round should claim the channel")
			}

			m.quotaState = func() quota.QuotaState { return state }
			m.refreshClaims(context.Background())
			if mr.Exists(youtubeclaim.ClaimKey("UCbusy")) {
				t.Fatalf("quota %s: every claim must be released so innertube serves the channel", state)
			}
			if _, ok := m.claimedOwner("UCbusy"); ok {
				t.Fatalf("quota %s: claimed set must be empty", state)
			}
			if elig.calls != 1 {
				t.Fatalf("quota %s: eligibility queried %d times, want 1 (no re-claim under pressure)", state, elig.calls)
			}
		})
	}
}

func TestRefreshClaims_NonLeaderDoesNothing(t *testing.T) {
	elig := &fakeEligibility{rows: []*models.StreamSource{row("ov1", "UCowned", "user-1")}}
	verifier := &fakeVerifier{verified: map[string]bool{"user-1|UCowned": true}}
	m, mr := newClaimsManager(t, elig, verifier)
	m.syncLeader = sourcemanager.NewLeadershipCoordinator("youtube", refusingLeadershipClient{}, time.Hour, zap.NewNop())
	// Claimed by this replica while it still held the lease; the new leader owns it now.
	m.claimed["UCprevious"] = "user-0"
	if err := mr.Set(youtubeclaim.ClaimKey("UCprevious"), "user-0"); err != nil {
		t.Fatal(err)
	}

	m.refreshClaims(context.Background())

	if elig.calls != 0 || verifier.calls != 0 {
		t.Fatalf("non-leader queried eligibility %d / verifier %d times, want 0", elig.calls, verifier.calls)
	}
	if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("non-leader must not write claims")
	}
	if !mr.Exists(youtubeclaim.ClaimKey("UCprevious")) {
		t.Fatal("non-leader must not release claims the current leader refreshes")
	}
	if _, ok := m.claimedOwner("UCprevious"); ok {
		t.Fatal("non-leader must clear its in-memory claimed set so its syncStreams serves nothing")
	}
}

func TestStop_ReleasesClaims(t *testing.T) {
	m, mr := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})
	m.stopChan = make(chan struct{})
	m.pollers = make(map[string]*Poller)
	m.claimed["UCowned"] = "user-1"
	if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-1"); err != nil {
		t.Fatal(err)
	}

	m.Stop()

	if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("a stopping replica must release its claims instead of leaving the channel dark until the TTL")
	}
	if _, ok := m.claimedOwner("UCowned"); ok {
		t.Fatal("claimed set must be empty after Stop")
	}
}

func TestDropOwner_ReleasesClaimAndForgetsVerdict(t *testing.T) {
	verifier := &fakeVerifier{}
	m, mr := newClaimsManager(t, &fakeEligibility{}, verifier)
	m.claimed["UCowned"] = "user-1"
	if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-1"); err != nil {
		t.Fatal(err)
	}

	m.dropOwner(context.Background(), "UCowned", "user-1", tokenRevoked)

	if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("claim must be released at once so innertube resumes on its next sync")
	}
	if _, ok := m.claimedOwner("UCowned"); ok {
		t.Fatal("dropped channel must leave the claimed set")
	}
	if len(verifier.forgotten) != 1 || verifier.forgotten[0] != "user-1|UCowned" {
		t.Fatalf("forgotten verdicts = %v, want [user-1|UCowned] so the next claim round re-checks the token", verifier.forgotten)
	}
}

// refreshClaims may hand the channel to a different owner between syncStreams' snapshot and
// the failing call; the new owner's claim is not this token's to drop.
func TestDropOwner_KeepsClaimOfDifferentOwner(t *testing.T) {
	verifier := &fakeVerifier{}
	m, mr := newClaimsManager(t, &fakeEligibility{}, verifier)
	m.claimed["UCowned"] = "user-2"
	if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-2"); err != nil {
		t.Fatal(err)
	}

	m.dropOwner(context.Background(), "UCowned", "user-1", tokenRevoked)

	if !mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("claim held for another owner must not be released")
	}
	if owner, _ := m.claimedOwner("UCowned"); owner != "user-2" {
		t.Fatalf("claimedOwner = %q, want user-2", owner)
	}
	if len(verifier.forgotten) != 1 || verifier.forgotten[0] != "user-1|UCowned" {
		t.Fatalf("forgotten verdicts = %v, want the failing token's [user-1|UCowned]", verifier.forgotten)
	}
}

var tokenRevoked = fmt.Errorf("failed to get token: failed to refresh token: %w",
	&oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}, ErrorCode: "invalid_grant"})

// A token-store read failing or the token endpoint being down says nothing about the owner's
// grant; bouncing the channel to innertube for it would flap it on every DB blip.
func TestDropOwner_KeepsClaimOnTransientError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"token_store", fmt.Errorf("failed to get token: failed to get token from store: %w", errors.New("conn closed"))},
		{"token_endpoint_503", fmt.Errorf("failed to get token: failed to refresh token: %w",
			&oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusServiceUnavailable}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := &fakeVerifier{}
			m, mr := newClaimsManager(t, &fakeEligibility{}, verifier)
			m.claimed["UCowned"] = "user-1"
			if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-1"); err != nil {
				t.Fatal(err)
			}

			m.dropOwner(context.Background(), "UCowned", "user-1", tc.err)

			if !mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
				t.Fatal("a transient token failure must not release the claim")
			}
			if owner, _ := m.claimedOwner("UCowned"); owner != "user-1" {
				t.Fatalf("claimedOwner = %q, want user-1", owner)
			}
			if len(verifier.forgotten) != 0 {
				t.Fatalf("forgotten verdicts = %v, want none", verifier.forgotten)
			}
		})
	}
}

// A claim round that read the verdict before dropOwner forgot it would otherwise claim the
// channel straight back with the rejected token.
func TestClaimRound_DropDuringVerifyIsNotReclaimed(t *testing.T) {
	elig := &fakeEligibility{}
	elig.set(row("ov1", "UCowned", "user-1"))
	verifier := &fakeVerifier{verified: map[string]bool{"user-1|UCowned": true}}
	m, mr := newClaimsManager(t, elig, verifier)
	m.claimed["UCowned"] = "user-1"
	if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-1"); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	verifier.onVerify = func() {
		once.Do(func() { m.dropOwner(context.Background(), "UCowned", "user-1", tokenRevoked) })
	}

	m.refreshClaims(context.Background())

	if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("a channel dropped during the round must not be re-claimed on the stale verdict")
	}
	if _, ok := m.claimedOwner("UCowned"); ok {
		t.Fatal("dropped channel must stay out of the claimed set")
	}

	m.refreshClaims(context.Background())
	if !mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("the next round re-verifies and may claim the channel again")
	}
}

// In production listenAndWait sits in WaitForNotification until its context ends, so Stop has to
// cancel that context and release the claims without waiting for every goroutine first.
func TestStop_ReleasesClaimsWhileListenerBlocked(t *testing.T) {
	m, mr := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})
	m.stopChan = make(chan struct{})
	m.pollers = make(map[string]*Poller)
	runCtx := m.beginRun(context.Background())
	m.claimed["UCowned"] = "user-1"
	if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-1"); err != nil {
		t.Fatal(err)
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		<-runCtx.Done()
	}()

	stopped := make(chan struct{})
	go func() {
		m.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop must cancel the run context instead of waiting on a blocked LISTEN forever")
	}
	if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("Stop must release the claim")
	}
}

func TestClaimedChannelSources_KeepsClaimedOwnerOnConnectedOverlays(t *testing.T) {
	m, _ := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})
	m.claimed["UCowned"] = "user-1"
	m.connectedOverlays["ov2"] = time.Now()

	channelSources, overlays := m.claimedChannelSources([]*models.StreamSource{
		row("ov1", "UCowned", "user-1"),
		row("ov2", "UCowned", "user-1"),
	})

	if got := len(channelSources["UCowned"]); got != 2 {
		t.Fatalf("UCowned sources = %d, want 2", got)
	}
	if _, ok := overlays["UCowned"]["ov1"]; !ok || len(overlays["UCowned"]) != 2 {
		t.Fatalf("connected overlays for UCowned = %v, want ov1 and ov2", overlays["UCowned"])
	}
}

func TestClaimedChannelSources_DropsUnclaimedChannel(t *testing.T) {
	m, _ := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})

	channelSources, overlays := m.claimedChannelSources([]*models.StreamSource{row("ov1", "UCfree", "user-1")})

	if len(channelSources) != 0 || len(overlays) != 0 {
		t.Fatalf("unclaimed channel kept: sources %v, overlays %v; innertube serves it", channelSources, overlays)
	}
}

func TestClaimedChannelSources_DropsOtherOwnerOnClaimedChannel(t *testing.T) {
	m, _ := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})
	m.claimed["UCshared"] = "real-owner"

	channelSources, _ := m.claimedChannelSources([]*models.StreamSource{
		row("ov-forged", "UCshared", "forger"),
		row("ov-real", "UCshared", "real-owner"),
	})

	got := channelSources["UCshared"]
	if len(got) != 1 || got[0].OwnerUserID != "real-owner" {
		t.Fatalf("UCshared sources = %v, want only the verified owner's row (its token is the one used)", got)
	}
}

func TestClaimedChannelSources_DropsDisconnectedOverlay(t *testing.T) {
	m, _ := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})
	m.claimed["UCowned"] = "user-1"

	channelSources, overlays := m.claimedChannelSources([]*models.StreamSource{
		row("ov1", "UCowned", "user-1"),
		row("ov-offline", "UCowned", "user-1"),
	})

	if got := channelSources["UCowned"]; len(got) != 1 || got[0].OverlayID != "ov1" {
		t.Fatalf("UCowned sources = %v, want only connected ov1", got)
	}
	if _, ok := overlays["UCowned"]["ov-offline"]; ok {
		t.Fatal("disconnected overlay must not be in the connected-overlay set")
	}
}

func TestClaimedChannelSources_KeepsOverlayInDisconnectDebounce(t *testing.T) {
	m, _ := newClaimsManager(t, &fakeEligibility{}, &fakeVerifier{})
	m.claimed["UCowned"] = "user-1"
	timer := time.AfterFunc(time.Hour, func() {})
	t.Cleanup(func() { timer.Stop() })
	m.disconnectDebounceTimers = map[string]*time.Timer{"ov-refreshing": timer}

	channelSources, _ := m.claimedChannelSources([]*models.StreamSource{row("ov-refreshing", "UCowned", "user-1")})

	if len(channelSources["UCowned"]) != 1 {
		t.Fatal("overlay inside the disconnect debounce must keep its claimed channel's poller")
	}
}
