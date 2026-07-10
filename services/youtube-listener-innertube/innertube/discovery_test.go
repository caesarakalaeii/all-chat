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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestDiscoverLiveStream_NetworkError(t *testing.T) {
	// Create discovery with no server (will cause network error)
	logger := zap.NewNop()
	client := &http.Client{}
	discovery := NewDiscovery(client, logger, ClientConfig{})

	ctx := context.Background()
	_, err := discovery.DiscoverLiveStream(ctx, "UC_invalid_channel", "", "")

	if err == nil {
		t.Error("expected network error, got nil")
	}
}

func TestExtractLiveChatContinuation(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected string
	}{
		{
			name: "finds continuation in liveChatRenderer with reloadContinuationData",
			data: map[string]interface{}{
				"contents": map[string]interface{}{
					"twoColumnWatchNextResults": map[string]interface{}{
						"conversationBar": map[string]interface{}{
							"liveChatRenderer": map[string]interface{}{
								"continuations": []interface{}{
									map[string]interface{}{
										"reloadContinuationData": map[string]interface{}{
											"continuation": "test_token_123",
										},
									},
								},
							},
						},
					},
				},
			},
			expected: "test_token_123",
		},
		{
			name: "finds continuation via fallback recursive search",
			data: map[string]interface{}{
				"someKey": map[string]interface{}{
					"liveChatRenderer": map[string]interface{}{
						"continuations": []interface{}{
							map[string]interface{}{
								"timedContinuationData": map[string]interface{}{
									"continuation":          "timed_token_456",
									"timeoutDurationMillis": float64(5000),
								},
							},
						},
					},
				},
			},
			expected: "timed_token_456",
		},
		{
			name:     "returns empty string when no liveChatRenderer",
			data:     map[string]interface{}{"someOtherKey": "value"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLiveChatContinuation(tt.data)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractContinuationFromLiveChatRenderer_PrefersMainContinuations(t *testing.T) {
	// The main continuations array token (150-200 chars) is accepted by
	// get_live_chat, while the shorter subMenuItem tokens (~32 chars) are
	// rejected with HTTP 400 as of March 2026. Main array must be preferred.
	renderer := map[string]interface{}{
		"continuations": []interface{}{
			map[string]interface{}{
				"reloadContinuationData": map[string]interface{}{
					"continuation": "main_continuation_token",
				},
			},
		},
		"header": map[string]interface{}{
			"liveChatHeaderRenderer": map[string]interface{}{
				"viewSelector": map[string]interface{}{
					"sortFilterSubMenuRenderer": map[string]interface{}{
						"subMenuItems": []interface{}{
							map[string]interface{}{
								"title": "Top chat",
								"continuation": map[string]interface{}{
									"reloadContinuationData": map[string]interface{}{
										"continuation": "top_chat_token",
									},
								},
							},
							map[string]interface{}{
								"title": "Live chat",
								"continuation": map[string]interface{}{
									"reloadContinuationData": map[string]interface{}{
										"continuation": "live_chat_token",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := extractContinuationFromLiveChatRenderer(renderer)
	if result != "main_continuation_token" {
		t.Errorf("expected main_continuation_token, got %q — main continuations must be preferred (subMenuItem tokens get HTTP 400)", result)
	}
}

func TestExtractContinuationFromLiveChatRenderer_FallsBackToMainContinuations(t *testing.T) {
	// When subMenuItems are absent, fall back to the main continuations array.
	renderer := map[string]interface{}{
		"continuations": []interface{}{
			map[string]interface{}{
				"reloadContinuationData": map[string]interface{}{
					"continuation": "fallback_token",
				},
			},
		},
	}

	result := extractContinuationFromLiveChatRenderer(renderer)
	if result != "fallback_token" {
		t.Errorf("expected fallback_token, got %q", result)
	}
}

func TestGetInitialContinuation_Success(t *testing.T) {
	// Mock server returning a valid /next API response with live chat continuation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"engagementPanels": []interface{}{
				map[string]interface{}{
					"engagementPanelSectionListRenderer": map[string]interface{}{
						"content": map[string]interface{}{
							"liveChatRenderer": map[string]interface{}{
								"continuations": []interface{}{
									map[string]interface{}{
										"reloadContinuationData": map[string]interface{}{
											"continuation": "initial_token_abc",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	t.Skip("Integration test - requires URL override capability in Discovery")
}

func TestGetInitialContinuation_NoToken(t *testing.T) {
	// Mock server returning response without continuation token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"videoDetails": map[string]interface{}{
				"videoId": "test123",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	t.Skip("Integration test - requires URL override capability in Discovery")
}

func TestDiscoverLiveStream_Success(t *testing.T) {
	t.Skip("Integration test - requires URL override capability")
}

func TestDiscoverLiveStream_NoStream(t *testing.T) {
	t.Skip("Integration test - requires URL override capability")
}

func TestDiscoverLiveStream_NotFound(t *testing.T) {
	// Mock server returning 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	logger := zap.NewNop()
	discovery := NewDiscovery(server.Client(), logger, ClientConfig{})

	ctx := context.Background()
	_, err := discovery.DiscoverLiveStream(ctx, "UCtest", "", "")

	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

// --- Unit tests for pure functions ---

func TestSelectStream(t *testing.T) {
	candidates := []LiveStreamCandidate{
		{VideoID: "vid1", Title: "Gaming Stream", ViewerCount: 100},
		{VideoID: "vid2", Title: "IRL Cooking Show", ViewerCount: 500},
		{VideoID: "vid3", Title: "Just Chatting", ViewerCount: 50},
	}

	t.Run("empty candidates returns error", func(t *testing.T) {
		_, err := SelectStream(nil, StrategyFirstFound, "")
		if err == nil {
			t.Error("expected error for empty candidates")
		}
	})

	t.Run("first_found returns first candidate", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyFirstFound, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid1" {
			t.Errorf("expected vid1, got %s", result.VideoID)
		}
	})

	t.Run("empty strategy defaults to most_viewers", func(t *testing.T) {
		// #473: the default flipped from first_found to most_viewers so
		// multi-stream channels land on the main (most-watched) stream
		// rather than candidates[0] (often a low/no-chat simulcast).
		result, err := SelectStream(candidates, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid2" {
			t.Errorf("expected vid2 (most_viewers default), got %s", result.VideoID)
		}
	})

	t.Run("most_viewers returns highest viewer count", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyMostViewers, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid2" {
			t.Errorf("expected vid2 (500 viewers), got %s (%d viewers)", result.VideoID, result.ViewerCount)
		}
	})

	t.Run("fewest_viewers returns lowest viewer count", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyFewestViewers, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid3" {
			t.Errorf("expected vid3 (50 viewers), got %s (%d viewers)", result.VideoID, result.ViewerCount)
		}
	})

	t.Run("fewest_viewers prefers known over unknown", func(t *testing.T) {
		mixed := []LiveStreamCandidate{
			{VideoID: "unknown", Title: "Unknown", ViewerCount: -1},
			{VideoID: "known", Title: "Known", ViewerCount: 200},
		}
		result, err := SelectStream(mixed, StrategyFewestViewers, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "known" {
			t.Errorf("expected known (200 viewers) over unknown (-1), got %s", result.VideoID)
		}
	})

	t.Run("fewest_viewers with all unknown returns first", func(t *testing.T) {
		unknowns := []LiveStreamCandidate{
			{VideoID: "u1", ViewerCount: -1},
			{VideoID: "u2", ViewerCount: -1},
		}
		result, err := SelectStream(unknowns, StrategyFewestViewers, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// When first has -1, second also -1 => second.ViewerCount >= 0 is false, so first stays
		// Actually: best starts as u1 (-1). For u2: best.ViewerCount < 0 is true, so best = u2.
		// This means with all unknowns, it picks the last one.
		if result.VideoID != "u2" {
			t.Errorf("expected u2 (last unknown), got %s", result.VideoID)
		}
	})

	t.Run("title_match finds matching title", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyTitleMatch, "cooking")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid2" {
			t.Errorf("expected vid2 (IRL Cooking Show), got %s", result.VideoID)
		}
	})

	t.Run("title_match is case insensitive", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyTitleMatch, "GAMING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid1" {
			t.Errorf("expected vid1 (Gaming Stream), got %s", result.VideoID)
		}
	})

	t.Run("title_match falls back to first when no match", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyTitleMatch, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid1" {
			t.Errorf("expected vid1 (fallback), got %s", result.VideoID)
		}
	})

	t.Run("title_match with empty term returns first", func(t *testing.T) {
		result, err := SelectStream(candidates, StrategyTitleMatch, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid1" {
			t.Errorf("expected vid1, got %s", result.VideoID)
		}
	})

	t.Run("unknown strategy falls back to most_viewers default", func(t *testing.T) {
		result, err := SelectStream(candidates, "unknown_strategy", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "vid2" {
			t.Errorf("expected vid2 (most_viewers default, #473), got %s", result.VideoID)
		}
	})

	t.Run("single candidate always returns it", func(t *testing.T) {
		single := []LiveStreamCandidate{{VideoID: "only", Title: "Only", ViewerCount: 42}}
		for _, strategy := range []string{StrategyFirstFound, StrategyMostViewers, StrategyFewestViewers, StrategyTitleMatch} {
			result, err := SelectStream(single, strategy, "nomatch")
			if err != nil {
				t.Fatalf("strategy %s: unexpected error: %v", strategy, err)
			}
			if result.VideoID != "only" {
				t.Errorf("strategy %s: expected only, got %s", strategy, result.VideoID)
			}
		}
	})

	t.Run("most_viewers with equal counts returns first", func(t *testing.T) {
		equal := []LiveStreamCandidate{
			{VideoID: "a", ViewerCount: 100},
			{VideoID: "b", ViewerCount: 100},
		}
		result, err := SelectStream(equal, StrategyMostViewers, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.VideoID != "a" {
			t.Errorf("expected a (first with equal viewers), got %s", result.VideoID)
		}
	})
}

func TestExtractViewerCount(t *testing.T) {
	t.Run("simpleText with comma separator", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"simpleText": "1,234 watching now",
			},
		}
		count := extractViewerCount(renderer)
		if count != 1234 {
			t.Errorf("expected 1234, got %d", count)
		}
	})

	t.Run("simpleText with dot separator", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"simpleText": "1.234 watching now",
			},
		}
		count := extractViewerCount(renderer)
		if count != 1234 {
			t.Errorf("expected 1234, got %d", count)
		}
	})

	t.Run("simpleText plain number", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"simpleText": "500 watching now",
			},
		}
		count := extractViewerCount(renderer)
		if count != 500 {
			t.Errorf("expected 500, got %d", count)
		}
	})

	t.Run("runs format", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"runs": []interface{}{
					map[string]interface{}{"text": "2,500"},
					map[string]interface{}{"text": " watching now"},
				},
			},
		}
		count := extractViewerCount(renderer)
		if count != 2500 {
			t.Errorf("expected 2500, got %d", count)
		}
	})

	t.Run("no viewCountText returns -1", func(t *testing.T) {
		renderer := map[string]interface{}{}
		count := extractViewerCount(renderer)
		if count != -1 {
			t.Errorf("expected -1, got %d", count)
		}
	})

	t.Run("empty simpleText returns -1", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"simpleText": "",
			},
		}
		count := extractViewerCount(renderer)
		if count != -1 {
			t.Errorf("expected -1, got %d", count)
		}
	})

	t.Run("non-numeric text returns -1", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"simpleText": "waiting...",
			},
		}
		count := extractViewerCount(renderer)
		if count != -1 {
			t.Errorf("expected -1, got %d", count)
		}
	})

	t.Run("viewCountText wrong type returns -1", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": "not a map",
		}
		count := extractViewerCount(renderer)
		if count != -1 {
			t.Errorf("expected -1, got %d", count)
		}
	})

	t.Run("empty runs returns -1", func(t *testing.T) {
		renderer := map[string]interface{}{
			"viewCountText": map[string]interface{}{
				"runs": []interface{}{},
			},
		}
		count := extractViewerCount(renderer)
		if count != -1 {
			t.Errorf("expected -1, got %d", count)
		}
	})
}

func TestExtractTitle(t *testing.T) {
	t.Run("standard runs format", func(t *testing.T) {
		renderer := map[string]interface{}{
			"title": map[string]interface{}{
				"runs": []interface{}{
					map[string]interface{}{"text": "My Cool Stream"},
				},
			},
		}
		title := extractTitle(renderer)
		if title != "My Cool Stream" {
			t.Errorf("expected 'My Cool Stream', got %q", title)
		}
	})

	t.Run("multiple runs concatenated", func(t *testing.T) {
		renderer := map[string]interface{}{
			"title": map[string]interface{}{
				"runs": []interface{}{
					map[string]interface{}{"text": "Part 1 "},
					map[string]interface{}{"text": "- Part 2"},
				},
			},
		}
		title := extractTitle(renderer)
		if title != "Part 1 - Part 2" {
			t.Errorf("expected 'Part 1 - Part 2', got %q", title)
		}
	})

	t.Run("no title key returns empty", func(t *testing.T) {
		renderer := map[string]interface{}{}
		title := extractTitle(renderer)
		if title != "" {
			t.Errorf("expected empty, got %q", title)
		}
	})

	t.Run("title wrong type returns empty", func(t *testing.T) {
		renderer := map[string]interface{}{
			"title": "plain string",
		}
		title := extractTitle(renderer)
		if title != "" {
			t.Errorf("expected empty, got %q", title)
		}
	})

	t.Run("no runs key returns empty", func(t *testing.T) {
		renderer := map[string]interface{}{
			"title": map[string]interface{}{},
		}
		title := extractTitle(renderer)
		if title != "" {
			t.Errorf("expected empty, got %q", title)
		}
	})

	t.Run("empty runs returns empty", func(t *testing.T) {
		renderer := map[string]interface{}{
			"title": map[string]interface{}{
				"runs": []interface{}{},
			},
		}
		title := extractTitle(renderer)
		if title != "" {
			t.Errorf("expected empty, got %q", title)
		}
	})

	t.Run("run without text field skipped", func(t *testing.T) {
		renderer := map[string]interface{}{
			"title": map[string]interface{}{
				"runs": []interface{}{
					map[string]interface{}{"bold": true},
					map[string]interface{}{"text": "Hello"},
				},
			},
		}
		title := extractTitle(renderer)
		if title != "Hello" {
			t.Errorf("expected 'Hello', got %q", title)
		}
	})
}

func TestCollectLiveCandidatesFromBrowse(t *testing.T) {
	t.Run("finds single live stream", func(t *testing.T) {
		data := map[string]interface{}{
			"videoId": "abc123",
			"title": map[string]interface{}{
				"runs": []interface{}{
					map[string]interface{}{"text": "Live Now!"},
				},
			},
			"viewCountText": map[string]interface{}{
				"simpleText": "1,000 watching now",
			},
			"thumbnailOverlays": []interface{}{
				map[string]interface{}{
					"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{
						"style": "LIVE",
					},
				},
			},
		}
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}
		if candidates[0].VideoID != "abc123" {
			t.Errorf("expected videoID abc123, got %s", candidates[0].VideoID)
		}
		if candidates[0].Title != "Live Now!" {
			t.Errorf("expected title 'Live Now!', got %q", candidates[0].Title)
		}
		if candidates[0].ViewerCount != 1000 {
			t.Errorf("expected 1000 viewers, got %d", candidates[0].ViewerCount)
		}
	})

	t.Run("skips non-LIVE overlays", func(t *testing.T) {
		data := map[string]interface{}{
			"videoId": "notlive",
			"thumbnailOverlays": []interface{}{
				map[string]interface{}{
					"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{
						"style": "DEFAULT",
					},
				},
			},
		}
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates for non-LIVE, got %d", len(candidates))
		}
	})

	t.Run("deduplicates by videoId", func(t *testing.T) {
		data := []interface{}{
			map[string]interface{}{
				"videoId": "dup1",
				"thumbnailOverlays": []interface{}{
					map[string]interface{}{
						"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{"style": "LIVE"},
					},
				},
			},
			map[string]interface{}{
				"videoId": "dup1",
				"thumbnailOverlays": []interface{}{
					map[string]interface{}{
						"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{"style": "LIVE"},
					},
				},
			},
		}
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 1 {
			t.Errorf("expected 1 deduplicated candidate, got %d", len(candidates))
		}
	})

	t.Run("finds nested live streams", func(t *testing.T) {
		data := map[string]interface{}{
			"contents": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"videoRenderer": map[string]interface{}{
							"videoId": "nested1",
							"title": map[string]interface{}{
								"runs": []interface{}{map[string]interface{}{"text": "Stream 1"}},
							},
							"thumbnailOverlays": []interface{}{
								map[string]interface{}{
									"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{"style": "LIVE"},
								},
							},
						},
					},
					map[string]interface{}{
						"videoRenderer": map[string]interface{}{
							"videoId": "nested2",
							"title": map[string]interface{}{
								"runs": []interface{}{map[string]interface{}{"text": "Stream 2"}},
							},
							"thumbnailOverlays": []interface{}{
								map[string]interface{}{
									"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{"style": "LIVE"},
								},
							},
						},
					},
				},
			},
		}
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 2 {
			t.Errorf("expected 2 candidates, got %d", len(candidates))
		}
	})

	t.Run("caps at 5 candidates", func(t *testing.T) {
		items := make([]interface{}, 10)
		for i := range items {
			items[i] = map[string]interface{}{
				"videoId": fmt.Sprintf("vid%d", i),
				"thumbnailOverlays": []interface{}{
					map[string]interface{}{
						"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{"style": "LIVE"},
					},
				},
			}
		}
		candidates := collectLiveCandidatesFromBrowse(items)
		if len(candidates) > 5 {
			t.Errorf("expected at most 5 candidates, got %d", len(candidates))
		}
	})

	t.Run("empty data returns nil", func(t *testing.T) {
		candidates := collectLiveCandidatesFromBrowse(map[string]interface{}{})
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates, got %d", len(candidates))
		}
	})

	t.Run("nil data returns nil", func(t *testing.T) {
		candidates := collectLiveCandidatesFromBrowse(nil)
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates, got %d", len(candidates))
		}
	})

	t.Run("no thumbnailOverlays key skips video", func(t *testing.T) {
		data := map[string]interface{}{
			"videoId": "no_overlay",
		}
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates, got %d", len(candidates))
		}
	})

	// The lockupViewModel schema replaced videoRenderer in YouTube's 2026 rollout.
	// Real-world example: channel UC8Zo7KTX9KFoly5x8DNu_zA streaming q_TkIzJAxrs.

	t.Run("finds live stream in lockupViewModel schema", func(t *testing.T) {
		data := newLockupCandidate("q_TkIzJAxrs", "TS HEAT IS FRYING ME | Just Chatting And Gaming | India Live", "4 watching", "THUMBNAIL_OVERLAY_BADGE_STYLE_LIVE")
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}
		if candidates[0].VideoID != "q_TkIzJAxrs" {
			t.Errorf("expected videoID q_TkIzJAxrs, got %s", candidates[0].VideoID)
		}
		if candidates[0].Title != "TS HEAT IS FRYING ME | Just Chatting And Gaming | India Live" {
			t.Errorf("unexpected title %q", candidates[0].Title)
		}
		if candidates[0].ViewerCount != 4 {
			t.Errorf("expected 4 viewers, got %d", candidates[0].ViewerCount)
		}
	})

	t.Run("skips non-LIVE lockupViewModel entries", func(t *testing.T) {
		// Recorded (replay) stream from the same channel: same shape but DEFAULT badge.
		data := newLockupCandidate("Z0tNZVht1-I", "Yesterday's stream", "3:19:55", "THUMBNAIL_OVERLAY_BADGE_STYLE_DEFAULT")
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates for non-LIVE lockup, got %d", len(candidates))
		}
	})

	t.Run("skips lockupViewModel without VIDEO contentType", func(t *testing.T) {
		data := newLockupCandidate("not_a_video", "Some playlist", "0 watching", "THUMBNAIL_OVERLAY_BADGE_STYLE_LIVE")
		data["contentType"] = "LOCKUP_CONTENT_TYPE_PLAYLIST"
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates for non-VIDEO lockup, got %d", len(candidates))
		}
	})

	t.Run("deduplicates across legacy and lockup schemas", func(t *testing.T) {
		// Same videoId appears in both a videoRenderer and a lockupViewModel in
		// the same response (YouTube serves both during the migration window).
		data := map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"videoRenderer": map[string]interface{}{
						"videoId": "shared_id",
						"title":   map[string]interface{}{"runs": []interface{}{map[string]interface{}{"text": "Legacy title"}}},
						"thumbnailOverlays": []interface{}{
							map[string]interface{}{"thumbnailOverlayTimeStatusRenderer": map[string]interface{}{"style": "LIVE"}},
						},
					},
				},
				newLockupCandidate("shared_id", "Lockup title", "10 watching", "THUMBNAIL_OVERLAY_BADGE_STYLE_LIVE"),
			},
		}
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 1 {
			t.Errorf("expected 1 deduplicated candidate, got %d", len(candidates))
		}
	})

	t.Run("lockup viewer count parses 'N watching' format", func(t *testing.T) {
		data := newLockupCandidate("v1", "t", "1,234 watching", "THUMBNAIL_OVERLAY_BADGE_STYLE_LIVE")
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 1 || candidates[0].ViewerCount != 1234 {
			t.Errorf("expected viewer count 1234, got %+v", candidates)
		}
	})

	t.Run("lockup viewer count returns -1 when row absent", func(t *testing.T) {
		// Premiere-style entry with metadata that doesn't say "watching".
		data := newLockupCandidate("v2", "t", "Premieres Tomorrow", "THUMBNAIL_OVERLAY_BADGE_STYLE_LIVE")
		candidates := collectLiveCandidatesFromBrowse(data)
		if len(candidates) != 1 || candidates[0].ViewerCount != -1 {
			t.Errorf("expected viewer count -1, got %+v", candidates)
		}
	})
}

// newLockupCandidate builds a lockupViewModel-shaped map mirroring YouTube's
// 2026 browse response schema, so tests document the structure the parser must
// handle. badgeStyle controls whether the entry registers as LIVE.
func newLockupCandidate(contentID, title, metadataText, badgeStyle string) map[string]interface{} {
	return map[string]interface{}{
		"contentId":   contentID,
		"contentType": "LOCKUP_CONTENT_TYPE_VIDEO",
		"contentImage": map[string]interface{}{
			"thumbnailViewModel": map[string]interface{}{
				"overlays": []interface{}{
					map[string]interface{}{
						"thumbnailBottomOverlayViewModel": map[string]interface{}{
							"badges": []interface{}{
								map[string]interface{}{
									"thumbnailBadgeViewModel": map[string]interface{}{
										"text":       "LIVE",
										"badgeStyle": badgeStyle,
									},
								},
							},
						},
					},
				},
			},
		},
		"metadata": map[string]interface{}{
			"lockupMetadataViewModel": map[string]interface{}{
				"title": map[string]interface{}{"content": title},
				"metadata": map[string]interface{}{
					"contentMetadataViewModel": map[string]interface{}{
						"metadataRows": []interface{}{
							map[string]interface{}{
								"metadataParts": []interface{}{
									map[string]interface{}{
										"text": map[string]interface{}{"content": metadataText},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
