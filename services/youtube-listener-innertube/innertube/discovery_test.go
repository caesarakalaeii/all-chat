package innertube

import (
	"context"
	"encoding/json"
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
	_, err := discovery.DiscoverLiveStream(ctx, "UC_invalid_channel")

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

func TestExtractContinuationFromLiveChatRenderer_PrefersLiveChatSubMenuItem(t *testing.T) {
	// When both the main continuations array and the "Live chat" subMenuItem
	// are present, the function must return the subMenuItem token to avoid
	// accidentally latching onto a "Top chat" continuation.
	renderer := map[string]interface{}{
		"continuations": []interface{}{
			map[string]interface{}{
				"reloadContinuationData": map[string]interface{}{
					"continuation": "top_chat_or_unknown_token",
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

	result := extractContinuationFromLiveChatRenderer(renderer, nil)
	if result != "live_chat_token" {
		t.Errorf("expected live_chat_token, got %q — subMenuItem should be preferred over main continuations", result)
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

	result := extractContinuationFromLiveChatRenderer(renderer, nil)
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
	_, err := discovery.DiscoverLiveStream(ctx, "UCtest")

	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}
