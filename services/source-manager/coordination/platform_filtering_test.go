package coordination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPlatformFromPodName(t *testing.T) {
	tests := []struct {
		name     string
		podName  string
		expected string
	}{
		{
			name:     "twitch-listener pod",
			podName:  "twitch-listener-abc123-xyz",
			expected: "twitch",
		},
		{
			name:     "twitch-eventsub-listener pod",
			podName:  "twitch-eventsub-listener-def456-abc",
			expected: "twitch-eventsub",
		},
		{
			name:     "kick-listener pod",
			podName:  "kick-listener-ghi789-def",
			expected: "kick",
		},
		{
			name:     "tiktok-listener pod",
			podName:  "tiktok-listener-jkl012-ghi",
			expected: "tiktok",
		},
		{
			name:     "youtube-listener pod",
			podName:  "youtube-listener-mno345-jkl",
			expected: "youtube",
		},
		{
			name:     "unknown pod name",
			podName:  "other-service-pqr678-mno",
			expected: "",
		},
		{
			name:     "actual twitch-listener pod name",
			podName:  "twitch-listener-844f6ccd77-fqbcn",
			expected: "twitch",
		},
		{
			name:     "actual twitch-eventsub pod name",
			podName:  "twitch-eventsub-listener-7bd68d5ccb-5t9bm",
			expected: "twitch-eventsub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPlatformFromPodName(tt.podName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroupPodsByPlatform(t *testing.T) {
	tests := []struct {
		name     string
		podIDs   []string
		expected map[string][]string
	}{
		{
			name: "single platform",
			podIDs: []string{
				"twitch-listener-abc-xyz",
				"twitch-listener-def-abc",
			},
			expected: map[string][]string{
				"twitch": {
					"twitch-listener-abc-xyz",
					"twitch-listener-def-abc",
				},
			},
		},
		{
			name: "multiple platforms",
			podIDs: []string{
				"twitch-listener-abc-xyz",
				"kick-listener-def-abc",
				"tiktok-listener-ghi-def",
			},
			expected: map[string][]string{
				"twitch": {"twitch-listener-abc-xyz"},
				"kick":   {"kick-listener-def-abc"},
				"tiktok": {"tiktok-listener-ghi-def"},
			},
		},
		{
			name: "twitch and twitch-eventsub",
			podIDs: []string{
				"twitch-listener-abc-xyz",
				"twitch-eventsub-listener-def-abc",
			},
			expected: map[string][]string{
				"twitch":         {"twitch-listener-abc-xyz"},
				"twitch-eventsub": {"twitch-eventsub-listener-def-abc"},
			},
		},
		{
			name: "mixed with unknown",
			podIDs: []string{
				"twitch-listener-abc-xyz",
				"unknown-service-def-abc",
				"kick-listener-ghi-def",
			},
			expected: map[string][]string{
				"twitch": {"twitch-listener-abc-xyz"},
				"kick":   {"kick-listener-ghi-def"},
			},
		},
		{
			name:     "empty list",
			podIDs:   []string{},
			expected: map[string][]string{},
		},
		{
			name: "actual production pod names",
			podIDs: []string{
				"twitch-listener-844f6ccd77-fqbcn",
				"twitch-eventsub-listener-7bd68d5ccb-5t9bm",
				"twitch-eventsub-listener-7bd68d5ccb-lkd2j",
				"kick-listener-66bbcfd9f4-7pk52",
				"tiktok-listener-6b4f7cd98f-vndn5",
			},
			expected: map[string][]string{
				"twitch": {"twitch-listener-844f6ccd77-fqbcn"},
				"twitch-eventsub": {
					"twitch-eventsub-listener-7bd68d5ccb-5t9bm",
					"twitch-eventsub-listener-7bd68d5ccb-lkd2j",
				},
				"kick":   {"kick-listener-66bbcfd9f4-7pk52"},
				"tiktok": {"tiktok-listener-6b4f7cd98f-vndn5"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupPodsByPlatform(tt.podIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlatformFilteringIntegration(t *testing.T) {
	// Simulate coordinator assignment with platform filtering
	podIDs := []string{
		"twitch-listener-1",
		"twitch-listener-2",
		"kick-listener-1",
	}

	grouped := groupPodsByPlatform(podIDs)

	// Verify twitch sources only go to twitch pods
	twitchPods, ok := grouped["twitch"]
	assert.True(t, ok, "twitch platform should exist")
	assert.Len(t, twitchPods, 2, "should have 2 twitch pods")

	// Verify kick sources only go to kick pods
	kickPods, ok := grouped["kick"]
	assert.True(t, ok, "kick platform should exist")
	assert.Len(t, kickPods, 1, "should have 1 kick pod")

	// Verify no cross-platform assignment
	assert.NotContains(t, twitchPods, "kick-listener-1")
	assert.NotContains(t, kickPods, "twitch-listener-1")
	assert.NotContains(t, kickPods, "twitch-listener-2")
}
