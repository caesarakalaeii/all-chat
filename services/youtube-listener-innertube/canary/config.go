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

// Package canary runs a permanently-demanded poller against channels whose
// chat is known to be busy, so that "the listener captured nothing" can be
// measured rather than inferred.
//
// Why this exists: an idle live chat and a broken continuation token both come
// back from get_live_chat as HTTP 200 with zero actions. Nothing inside a
// response distinguishes them. The previous detector worked around that by
// assuming that across every demanded stream *somebody* is always talking —
// a statistical claim about the user base, not a measurement, and it produced
// alerts nobody trusted. A channel that is live 24/7 with continuously busy
// chat removes the assumption: if the canary captures nothing, the capture
// path is broken, full stop.
//
// Deliberately NOT an overlay. A real overlay would need demand, which comes
// from an api-gateway WebSocket, so it would need a headless client running
// forever; worse, the canary's chat volume would then flow through
// message-processor, emote enrichment, the AllChatPlatformMessagesEmpty ratio
// and the DAU/WAU/MAU aggregates, permanently skewing every one of them to fix
// a single alert. Instead the canary polls through the same code path as
// production traffic, counts what it captured, and drops it.
package canary

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Target is one canary channel.
//
// VideoID is pinned rather than discovered because 24/7 channels frequently run
// several concurrent streams, and the `first_found` selection bug (#473) would
// then let the canary silently attach to a low-traffic simulcast with no chat —
// recreating the false alert with extra steps. Rediscovery still happens, but
// only when the pinned video stops working (see Canary.run).
type Target struct {
	ChannelID string
	VideoID   string
}

// Config configures the canary poller.
type Config struct {
	// Enabled gates the whole subsystem. Off means no goroutine, no polls and
	// no canary metrics — the alert side detects that via
	// YouTubeInnerTubeCanaryDown, so turning it off is loud rather than silent.
	Enabled bool

	// Targets are the canary channels. Two or more is the intended
	// configuration: any single channel can flip its chat to members-only or
	// slow mode without warning, and a one-channel canary would page us for
	// that. The alert side treats the canaries as a set.
	Targets []Target

	// PollInterval is the floor between get_live_chat calls, as for a normal
	// poller. YouTube's own recommended timeout wins when it is longer.
	PollInterval time.Duration

	// RediscoverInterval is how often a target that is not currently polling
	// (pinned video ended, continuation could not be fetched) retries. It is
	// deliberately slow: the canary is a detector, not a user-facing stream,
	// and hammering YouTube to re-pin it would be its own incident.
	RediscoverInterval time.Duration
}

// CorrelatedChannels returns the channel IDs that more than one target is
// pinned to, sorted, or nil when every target sits on its own channel.
//
// Two pins on one channel look like redundancy but are not: members-only and
// slow mode are channel-wide settings, so a single moderation decision silences
// every stream that channel owns simultaneously. That is precisely the event
// the second canary exists to survive, which is why the configuration is worth
// reporting even though it is not an error.
func (c Config) CorrelatedChannels() []string {
	counts := make(map[string]int, len(c.Targets))
	for _, t := range c.Targets {
		counts[t.ChannelID]++
	}
	var shared []string
	for channelID, n := range counts {
		if n > 1 {
			shared = append(shared, channelID)
		}
	}
	sort.Strings(shared)
	return shared
}

// Defaults applied by ParseConfig when the corresponding env var is unset.
const (
	DefaultPollInterval       = 2 * time.Second
	DefaultRediscoverInterval = 10 * time.Minute
)

// ParseTargets parses the YOUTUBE_CANARY_CHANNELS format: a comma-separated
// list of `channelID:videoID` pairs, e.g.
//
//	UCSJ4gkVC6NrvII8umztf0Ow:jfKfPfyJRdk,UCQM2AaEbwOhkg4Mn2P0AuPQ:Na0w3Mz46GA
//
// The video ID is mandatory. A pair without one is rejected instead of falling
// back to discovery: an unpinned canary is exactly the multi-stream selection
// hazard this design exists to avoid, and failing at startup is better than
// discovering it during an outage.
func ParseTargets(raw string) ([]Target, error) {
	var targets []Target
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		channelID, videoID, ok := strings.Cut(entry, ":")
		channelID = strings.TrimSpace(channelID)
		videoID = strings.TrimSpace(videoID)
		if !ok || channelID == "" || videoID == "" {
			return nil, fmt.Errorf("canary target %q: want channelID:videoID", entry)
		}
		targets = append(targets, Target{ChannelID: channelID, VideoID: videoID})
	}
	return targets, nil
}

// ParseConfig builds a Config from the environment accessor `env` (typically
// listener.Env), so the canary can be switched off in production without a code
// change.
//
//	YOUTUBE_CANARY_ENABLED             "true" to run it (default off)
//	YOUTUBE_CANARY_CHANNELS            channelID:videoID[,channelID:videoID...]
//	YOUTUBE_CANARY_POLL_INTERVAL       Go duration, default 2s
//	YOUTUBE_CANARY_REDISCOVER_INTERVAL Go duration, default 10m
//
// Enabled with no targets is a configuration error rather than a silent no-op:
// it would leave YouTubeInnerTubeCanaryDown firing forever with no explanation.
func ParseConfig(env func(key, fallback string) string) (Config, error) {
	cfg := Config{
		Enabled:            strings.EqualFold(env("YOUTUBE_CANARY_ENABLED", "false"), "true"),
		PollInterval:       DefaultPollInterval,
		RediscoverInterval: DefaultRediscoverInterval,
	}

	targets, err := ParseTargets(env("YOUTUBE_CANARY_CHANNELS", ""))
	if err != nil {
		return Config{}, err
	}
	cfg.Targets = targets

	if raw := env("YOUTUBE_CANARY_POLL_INTERVAL", ""); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("YOUTUBE_CANARY_POLL_INTERVAL %q: %w", raw, err)
		}
		cfg.PollInterval = d
	}
	if raw := env("YOUTUBE_CANARY_REDISCOVER_INTERVAL", ""); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("YOUTUBE_CANARY_REDISCOVER_INTERVAL %q: %w", raw, err)
		}
		cfg.RediscoverInterval = d
	}

	if cfg.Enabled && len(cfg.Targets) == 0 {
		return Config{}, fmt.Errorf("YOUTUBE_CANARY_ENABLED is true but YOUTUBE_CANARY_CHANNELS is empty")
	}

	return cfg, nil
}
