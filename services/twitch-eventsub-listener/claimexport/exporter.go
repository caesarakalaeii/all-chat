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

// Package claimexport mirrors the live EventSub chat-ownership claim set (ADR-0015)
// into a Prometheus gauge so dashboards can tell which Twitch channels EventSub is
// serving right now. The IRC↔EventSub partition is a live Redis claim, not a metric;
// without this exporter the only Prometheus signal for "on IRC" is message activity,
// which counts brief claim-lapse flaps (a quiet EventSub channel whose TTL expires for
// a moment, gets one IRC message, then re-claims) and so reports already-migrated
// channels as still needing migration.
package claimexport

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/shared/twitchchat"
)

// OwnedRefreshInterval is how often the live claim set is mirrored into the gauge.
// Comfortably below the claim TTL (5m) so the gauge tracks ownership closely, and a
// small multiple of ClaimRefreshInterval so it costs at most a couple of Redis SCANs
// per minute per replica.
const OwnedRefreshInterval = 30 * time.Second

// chatOwned is 1 for each Twitch channel whose chat the EventSub listener currently
// owns (a live ADR-0015 chat-ownership claim). The login is exposed under the
// "channel_id" label so it lines up with listener_messages_received_total.channel_id
// (which is also the lower-cased login) — letting the migration dashboard subtract
// already-migrated channels with `… unless on (channel_id) (listener_eventsub_chat_owned)`.
var chatOwned = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "listener_eventsub_chat_owned",
	Help: "1 per Twitch channel whose chat the EventSub listener currently owns (live ADR-0015 chat-ownership claim). channel_id is the lower-cased login.",
}, []string{"service", "channel_id"})

// ExportOwnedChannels mirrors the live chat-ownership claim set into the chatOwned
// gauge every OwnedRefreshInterval until ctx is done. It reads the SAME Redis claim
// set the IRC listener consults (twitchchat.ClaimStore.ClaimedLogins), so the gauge
// is the authoritative "served by EventSub right now" signal rather than a proxy.
//
// Safe to run on every replica: writes are idempotent and duplicate series (one per
// pod) collapse under `max by (channel_id)` / `unless on (channel_id)` in PromQL. A
// nil store (e.g. no Redis in tests) makes this a no-op. On a Redis read error the
// gauge is left unchanged rather than cleared, so a transient blip can't make every
// migrated channel briefly reappear as unmigrated.
func ExportOwnedChannels(ctx context.Context, claims *twitchchat.ClaimStore, service string, logger *zap.Logger) {
	if claims == nil {
		return
	}

	prev := syncOnce(ctx, claims, service, map[string]struct{}{}, logger) // populate immediately
	ticker := time.NewTicker(OwnedRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prev = syncOnce(ctx, claims, service, prev, logger)
		}
	}
}

// syncOnce reconciles the gauge with the live claim set for one cycle and returns the
// new set (to become the next call's prev). Owned logins are set to 1; logins present
// in prev but no longer claimed have their series removed so the gauge never reports
// stale ownership. On a Redis error the gauge is left untouched and prev is returned
// unchanged, so a transient failure can't flap every channel back to "unmigrated".
func syncOnce(ctx context.Context, claims *twitchchat.ClaimStore, service string, prev map[string]struct{}, logger *zap.Logger) map[string]struct{} {
	owned, err := claims.ClaimedLogins(ctx)
	if err != nil {
		logger.Warn("chat-owned exporter: failed to read claims; leaving gauge unchanged", zap.Error(err))
		return prev
	}
	for login := range owned {
		chatOwned.WithLabelValues(service, login).Set(1)
	}
	for login := range prev {
		if _, still := owned[login]; !still {
			chatOwned.DeleteLabelValues(service, login)
		}
	}
	return owned
}
