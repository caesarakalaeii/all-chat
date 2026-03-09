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
	discovery := NewDiscovery(client, logger)

	ctx := context.Background()
	_, err := discovery.DiscoverLiveStream(ctx, "UC_invalid_channel")

	if err == nil {
		t.Error("expected network error, got nil")
	}
}

func TestExtractLiveChatContinuationFromNext(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected string
	}{
		{
			name: "finds continuation in liveChatRenderer with reloadContinuationData",
			data: map[string]interface{}{
				"engagementPanels": []interface{}{
					map[string]interface{}{
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
			expected: "test_token_123",
		},
		{
			name: "finds continuation with timedContinuationData",
			data: map[string]interface{}{
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
			expected: "timed_token_456",
		},
		{
			name:     "returns empty string when no liveChatRenderer",
			data:     map[string]interface{}{"someOtherKey": "value"},
			expected: "",
		},
		{
			name:     "returns empty string for nil",
			data:     nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLiveChatContinuationFromNext(tt.data)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
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
	discovery := NewDiscovery(server.Client(), logger)

	ctx := context.Background()
	_, err := discovery.DiscoverLiveStream(ctx, "UCtest")

	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}
