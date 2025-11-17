package clients

import "testing"

func TestBuildTwitchEmoteURLHandlesTemplates(t *testing.T) {
	emote := twitchChatEmote{
		ID:        "305954156",
		Format:    []string{"static"},
		ThemeMode: []string{"dark"},
		Scale:     []string{"3.0"},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "single braces",
			template: "https://static-cdn.jtvnw.net/emoticons/v2/{id}/{format}/{theme_mode}/{scale}",
			expected: "https://static-cdn.jtvnw.net/emoticons/v2/305954156/static/dark/3.0",
		},
		{
			name:     "double braces",
			template: "https://static-cdn.jtvnw.net/emoticons/v2/{{id}}/{{format}}/{{theme_mode}}/{{scale}}",
			expected: "https://static-cdn.jtvnw.net/emoticons/v2/305954156/static/dark/3.0",
		},
	}

	for _, tt := range tests {
		got := buildTwitchEmoteURL(tt.template, emote)
		if got != tt.expected {
			t.Fatalf("%s: expected %s, got %s", tt.name, tt.expected, got)
		}
	}
}
