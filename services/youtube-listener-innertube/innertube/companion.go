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

package innertube

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"go.uber.org/zap"
)

// Vertical companion modes, from a YouTube source's config.vertical_companion.
//
// Streaming landscape and vertical at the same time gives a channel two separate
// broadcasts, each with its own chat. Single-stream strategies poll only one of them,
// so the other chat never reached the overlay. A companion is the one extra broadcast
// merged next to the selected stream. It is free (unlike the premium "all" strategy,
// which follows any number of independent broadcasts, e.g. 24/7 music channels).
const (
	// CompanionAuto merges a second live broadcast only when its title is the main
	// stream's title (ignoring vertical markers) or looks vertical. It is the default
	// (empty config value).
	CompanionAuto = ""
	// CompanionOn merges one second live broadcast whatever its title says, for
	// streamers whose vertical title carries no marker.
	CompanionOn = "on"
	// CompanionOff never merges a second broadcast.
	CompanionOff = "off"
)

// ValidCompanionModes lists the accepted config.vertical_companion values.
var ValidCompanionModes = map[string]bool{CompanionAuto: true, CompanionOn: true, CompanionOff: true}

// verticalTitleMarker matches the words streamers and multistream tools put in a
// vertical broadcast's title. YouTube itself does not mark the broadcast: in the
// browse, watch-next, player and reel responses both broadcasts of a dual-format
// stream are indistinguishable (checked 2026-11-14), so the title is the only signal.
// Seen in production: "(Portrait)", "- Vertical", "| Vertical Live", "[PION]"
// (Polish), "#shortslive".
var verticalTitleMarker = regexp.MustCompile(`(?i)\bverti(cal|cale|kal)\b|\bportrait\b|\bhochformat\b|\bhochkant\b|\bpion(owo)?\b|#shortslive|#verticallive`)

// LooksVertical reports whether a broadcast title marks it as a vertical stream.
func LooksVertical(title string) bool {
	return verticalTitleMarker.MatchString(title)
}

// broadcastTitleKey reduces a title to what identifies the broadcast: vertical markers
// removed, case and everything but letters and digits dropped. Dual-format setups either
// reuse the title or append a marker ("Proviamo Sniper Elite 4!" next to
// "Proviamo Sniper Elite 4! - Vertical"), so both reduce to the same key.
func broadcastTitleKey(title string) string {
	stripped := verticalTitleMarker.ReplaceAllString(strings.ToLower(title), " ")
	var b strings.Builder
	for _, r := range stripped {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SameBroadcastTitle reports whether two broadcasts carry the same title once vertical
// markers are ignored: the strongest sign that one is the other's vertical twin.
func SameBroadcastTitle(a, b string) bool {
	ka := broadcastTitleKey(a)
	return ka != "" && ka == broadcastTitleKey(b)
}

// preferLandscape drops vertical-looking candidates as long as at least one other
// candidate remains. Without it "most viewers" sometimes picked the vertical broadcast
// as the main stream, and the landscape chat was the one that went missing.
func preferLandscape(candidates []LiveStreamCandidate) []LiveStreamCandidate {
	landscape := make([]LiveStreamCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !LooksVertical(c.Title) {
			landscape = append(landscape, c)
		}
	}
	if len(landscape) == 0 {
		return candidates
	}
	return landscape
}

// SelectCompanion picks the broadcast to merge next to main, or reports false. In order:
// a broadcast with main's title (ignoring vertical markers), then a vertical-looking
// title, then, with mode "on" only, any other live broadcast. Ties go to the
// most-watched one.
func SelectCompanion(candidates []LiveStreamCandidate, main LiveStreamCandidate, mode string) (LiveStreamCandidate, bool) {
	if mode == CompanionOff {
		return LiveStreamCandidate{}, false
	}
	var sameTitle, vertical, other []LiveStreamCandidate
	for _, c := range candidates {
		switch {
		case c.VideoID == main.VideoID:
		case SameBroadcastTitle(c.Title, main.Title):
			sameTitle = append(sameTitle, c)
		case LooksVertical(c.Title):
			vertical = append(vertical, c)
		default:
			other = append(other, c)
		}
	}
	pool := sameTitle
	if len(pool) == 0 {
		pool = vertical
	}
	if len(pool) == 0 && mode == CompanionOn {
		pool = other
	}
	if len(pool) == 0 {
		return LiveStreamCandidate{}, false
	}
	best := pool[0]
	for _, c := range pool[1:] {
		if c.ViewerCount > best.ViewerCount {
			best = c
		}
	}
	return best, true
}

// DiscoverLiveStreams selects a channel's main live stream with a single-stream
// strategy and, per companionMode, its vertical companion. The main stream's video
// ID comes first; the companion, if any, second.
func (d *Discovery) DiscoverLiveStreams(ctx context.Context, channelID, strategy, matchTerm, companionMode string) ([]string, error) {
	if strategy == "" {
		strategy = StrategyMostViewers
	}

	candidates, err := d.browseLiveCandidates(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no live stream found for channel %s", channelID)
	}

	main, err := SelectStream(candidates, strategy, matchTerm)
	if err != nil {
		return nil, fmt.Errorf("stream selection failed for channel %s: %w", channelID, err)
	}
	videoIDs := []string{main.VideoID}

	fields := []zap.Field{
		zap.String("channel_id", channelID),
		zap.String("video_id", main.VideoID),
		zap.String("title", main.Title),
		zap.Int("viewer_count", main.ViewerCount),
		zap.Int("candidates", len(candidates)),
		zap.String("strategy", strategy),
	}
	if companion, ok := SelectCompanion(candidates, main, companionMode); ok {
		videoIDs = append(videoIDs, companion.VideoID)
		fields = append(fields,
			zap.String("companion_video_id", companion.VideoID),
			zap.String("companion_title", companion.Title),
		)
	}
	d.logger.Info("discovered live stream", fields...)
	return videoIDs, nil
}
