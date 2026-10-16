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

package alerts

// Goal and list overlays (issue #955, ADR-0072): their configs, the Redis keys
// the alert-processor writes and overlay-manager reads, and the two frames
// that ride overlay:{id}:alerts next to alert envelopes.

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/redis/go-redis/v9"
)

// goalMetricPlatform pairs each goal metric with the one platform that has it.
// followers/subscribers are API-polled; bits/super_chat are event counters.
var goalMetricPlatform = map[string]string{
	"followers":   "twitch",
	"subscribers": "youtube",
	"bits":        "twitch",
	"super_chat":  "youtube",
}

const maxGoalTitleLen = 200

// IsPolledGoalMetric reports whether the metric's current value comes from a
// platform API poll rather than from counted events.
func IsPolledGoalMetric(metric string) bool {
	return metric == "followers" || metric == "subscribers"
}

// GoalConfig is one goal overlay's goal_configs row.
type GoalConfig struct {
	Metric        string `json:"metric"`
	Platform      string `json:"platform"`
	Target        int64  `json:"target"`
	StartingValue int64  `json:"starting_value"`
	Title         string `json:"title"`
}

// Validate rejects anything the goal_configs table would accept but the goal
// renderer or the poller cannot serve.
func (g GoalConfig) Validate() error {
	platform, ok := goalMetricPlatform[g.Metric]
	if !ok {
		return fmt.Errorf("metric must be one of followers, subscribers, bits, super_chat")
	}
	if g.Platform != platform {
		return fmt.Errorf("metric %s is only available on %s", g.Metric, platform)
	}
	if g.Target <= 0 {
		return fmt.Errorf("target must be greater than 0")
	}
	if g.StartingValue < 0 {
		return fmt.Errorf("starting_value must not be negative")
	}
	if len(g.Title) > maxGoalTitleLen {
		return fmt.Errorf("title must be at most %d characters", maxGoalTitleLen)
	}
	return nil
}

// GoalKey holds a goal's live value: the last polled count for polled
// metrics, the summed event amount (excluding starting_value) for counters.
func GoalKey(overlayID, metric string) string {
	return fmt.Sprintf("overlay:%s:goal:%s", overlayID, metric)
}

// GoalStatusKey holds why the last poll could not produce a count
// ("reconnect", "hidden"); absent when the last poll succeeded.
func GoalStatusKey(overlayID string) string {
	return fmt.Sprintf("overlay:%s:goal-status", overlayID)
}

// GoalFrame is the {type:"goal"} payload on overlay:{id}:alerts.
type GoalFrame struct {
	Type      string  `json:"type"` // always "goal"
	OverlayID string  `json:"overlay_id"`
	Goal      string  `json:"goal"` // the metric
	Current   float64 `json:"current"`
	Target    int64   `json:"target"`
	Title     string  `json:"title"`
}

// ListCategories are the leaderboards the alert-processor keeps (ADR-0068).
var ListCategories = []string{"bits", "gifts", "super_chat"}

// MaxListLimit caps a list overlay's ranks; list_update frames carry this many.
const MaxListLimit = 25

// ListConfig is one list overlay's list_configs row.
type ListConfig struct {
	Category string `json:"category"`
	Limit    int    `json:"limit"`
	Title    string `json:"title"`
}

// Validate rejects unknown boards and out-of-range limits.
func (l ListConfig) Validate() error {
	if !IsListCategory(l.Category) {
		return fmt.Errorf("category must be one of %s", strings.Join(ListCategories, ", "))
	}
	if l.Limit < 1 || l.Limit > MaxListLimit {
		return fmt.Errorf("limit must be between 1 and %d", MaxListLimit)
	}
	if len(l.Title) > maxGoalTitleLen {
		return fmt.Errorf("title must be at most %d characters", maxGoalTitleLen)
	}
	return nil
}

// IsListCategory reports whether category names a leaderboard.
func IsListCategory(category string) bool {
	return slices.Contains(ListCategories, category)
}

// LeaderboardKey is the sorted set: score = summed amount, member =
// {platform}:{user_id}.
func LeaderboardKey(overlayID, category string) string {
	return fmt.Sprintf("overlay:%s:leaderboard:%s", overlayID, category)
}

// LeaderboardNamesKey is the hash member -> latest display name, written next
// to every score so a board can render names, not platform user ids.
func LeaderboardNamesKey(overlayID, category string) string {
	return LeaderboardKey(overlayID, category) + ":names"
}

// LeaderboardEntry is one ranked row.
type LeaderboardEntry struct {
	Rank     int     `json:"rank"`
	Name     string  `json:"name"`
	Platform string  `json:"platform"`
	Amount   float64 `json:"amount"`
}

// ListUpdateFrame is the {type:"list_update"} payload on overlay:{id}:alerts.
// The alert-processor sends one per changed board (every board of a list
// overlay is scored, whichever it shows). Limit and Title are set only by an
// owner's save, which is also what tells the renderer to switch to Category.
type ListUpdateFrame struct {
	Type      string             `json:"type"` // always "list_update"
	OverlayID string             `json:"overlay_id"`
	Category  string             `json:"category"`
	Entries   []LeaderboardEntry `json:"entries"`
	Limit     int                `json:"limit,omitempty"`
	Title     string             `json:"title,omitempty"`
}

// TopEntries reads the top limit members of a board, highest score first.
func TopEntries(ctx context.Context, rdb redis.UniversalClient, overlayID, category string, limit int) ([]LeaderboardEntry, error) {
	scored, err := rdb.ZRevRangeWithScores(ctx, LeaderboardKey(overlayID, category), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("read leaderboard %s/%s: %w", overlayID, category, err)
	}
	entries := make([]LeaderboardEntry, 0, len(scored))
	if len(scored) == 0 {
		return entries, nil
	}
	members := make([]string, len(scored))
	for i, z := range scored {
		members[i], _ = z.Member.(string)
	}
	names, err := rdb.HMGet(ctx, LeaderboardNamesKey(overlayID, category), members...).Result()
	if err != nil {
		return nil, fmt.Errorf("read leaderboard names %s/%s: %w", overlayID, category, err)
	}
	for i, z := range scored {
		platform, userID, _ := strings.Cut(members[i], ":")
		name, _ := names[i].(string)
		if name == "" {
			name = userID
		}
		entries = append(entries, LeaderboardEntry{Rank: i + 1, Name: name, Platform: platform, Amount: z.Score})
	}
	return entries, nil
}
