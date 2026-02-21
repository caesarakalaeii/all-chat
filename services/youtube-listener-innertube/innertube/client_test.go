package innertube

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		opts     ClientOptions
		wantKey  string
		wantTimeout time.Duration
	}{
		{
			name:     "default options",
			opts:     ClientOptions{},
			wantKey:  DefaultAPIKey,
			wantTimeout: DefaultTimeout,
		},
		{
			name: "custom API key",
			opts: ClientOptions{
				APIKey: "custom-key",
			},
			wantKey:  "custom-key",
			wantTimeout: DefaultTimeout,
		},
		{
			name: "custom timeout",
			opts: ClientOptions{
				Timeout: 5 * time.Second,
			},
			wantKey:  DefaultAPIKey,
			wantTimeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.opts)
			if client.apiKey != tt.wantKey {
				t.Errorf("apiKey = %v, want %v", client.apiKey, tt.wantKey)
			}
			if client.httpClient.Timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", client.httpClient.Timeout, tt.wantTimeout)
			}
		})
	}
}

func TestGetLiveChatReplay(t *testing.T) {
	tests := []struct {
		name           string
		continuation   string
		responseStatus int
		responseBody   interface{}
		wantErr        bool
		wantErrType    ErrorType
		wantActions    int
	}{
		{
			name:         "successful request",
			continuation: "test-continuation-token",
			responseStatus: http.StatusOK,
			responseBody: LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Actions: []ChatAction{
							{
								AddChatItemAction: &AddChatItemAction{
									Item: ChatItem{
										LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
											Message: MessageContent{
												Runs: []MessageRun{{Text: "Test message"}},
											},
											AuthorName: SimpleText{SimpleText: "Test User"},
											AuthorExternalChannelID: "UC123",
											TimestampUsec: "1234567890000000",
										},
									},
								},
							},
						},
						Continuations: []Continuation{
							{
								TimedContinuationData: &TimedContinuationData{
									Continuation: "next-token",
									TimeoutDurationMillis: 2000,
								},
							},
						},
					},
				},
			},
			wantErr:     false,
			wantActions: 1,
		},
		{
			name:           "empty continuation",
			continuation:   "",
			responseStatus: http.StatusOK,
			wantErr:        true,
		},
		{
			name:           "401 unauthorized",
			continuation:   "test-token",
			responseStatus: http.StatusUnauthorized,
			responseBody:   map[string]string{"error": "unauthorized"},
			wantErr:        true,
			wantErrType:    ErrorTypeFatal,
		},
		{
			name:           "404 not found",
			continuation:   "test-token",
			responseStatus: http.StatusNotFound,
			responseBody:   map[string]string{"error": "not found"},
			wantErr:        true,
			wantErrType:    ErrorTypeFatal,
		},
		{
			name:           "429 rate limit",
			continuation:   "test-token",
			responseStatus: http.StatusTooManyRequests,
			responseBody:   map[string]string{"error": "rate limited"},
			wantErr:        true,
			wantErrType:    ErrorTypeTransient,
		},
		{
			name:           "500 server error",
			continuation:   "test-token",
			responseStatus: http.StatusInternalServerError,
			responseBody:   map[string]string{"error": "server error"},
			wantErr:        true,
			wantErrType:    ErrorTypeTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Test empty continuation early
			if tt.continuation == "" {
				client := NewClient(ClientOptions{})
				_, err := client.GetLiveChatReplay(ctx, tt.continuation)
				if (err != nil) != tt.wantErr {
					t.Errorf("GetLiveChatReplay() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			// For non-empty continuation tests, verify error classification
			if tt.responseStatus != http.StatusOK {
				httpErr := &HTTPStatusError{
					StatusCode: tt.responseStatus,
					Body:       "test error",
				}

				errType := ClassifyError(httpErr)
				if errType != tt.wantErrType {
					t.Errorf("ClassifyError() = %v, want %v", errType, tt.wantErrType)
				}

				if IsFatalError(httpErr) != (tt.wantErrType == ErrorTypeFatal) {
					t.Errorf("IsFatalError() = %v, want %v", IsFatalError(httpErr), tt.wantErrType == ErrorTypeFatal)
				}
				if IsTransientError(httpErr) != (tt.wantErrType == ErrorTypeTransient) {
					t.Errorf("IsTransientError() = %v, want %v", IsTransientError(httpErr), tt.wantErrType == ErrorTypeTransient)
				}
			}
		})
	}
}

func TestExtractContinuation(t *testing.T) {
	client := NewClient(ClientOptions{})

	tests := []struct {
		name string
		resp *LiveChatResponse
		want string
	}{
		{
			name: "nil response",
			resp: nil,
			want: "",
		},
		{
			name: "no continuations",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{},
					},
				},
			},
			want: "",
		},
		{
			name: "timed continuation",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{
							{
								TimedContinuationData: &TimedContinuationData{
									Continuation: "timed-token",
									TimeoutDurationMillis: 2000,
								},
							},
						},
					},
				},
			},
			want: "timed-token",
		},
		{
			name: "invalidation continuation",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{
							{
								InvalidationContinuationData: &InvalidationContinuationData{
									Continuation: "invalidation-token",
									TimeoutDurationMillis: 3000,
								},
							},
						},
					},
				},
			},
			want: "invalidation-token",
		},
		{
			name: "replay continuation",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{
							{
								LiveChatReplayContinuationData: &LiveChatReplayContinuationData{
									Continuation: "replay-token",
									TimeUntilLastMessageMsec: 1000,
								},
							},
						},
					},
				},
			},
			want: "replay-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.ExtractContinuation(tt.resp)
			if got != tt.want {
				t.Errorf("ExtractContinuation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPollInterval(t *testing.T) {
	client := NewClient(ClientOptions{})

	tests := []struct {
		name string
		resp *LiveChatResponse
		want time.Duration
	}{
		{
			name: "nil response",
			resp: nil,
			want: 0,
		},
		{
			name: "no continuations",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{},
					},
				},
			},
			want: 0,
		},
		{
			name: "timed continuation with timeout",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{
							{
								TimedContinuationData: &TimedContinuationData{
									Continuation: "token",
									TimeoutDurationMillis: 2000,
								},
							},
						},
					},
				},
			},
			want: 2 * time.Second,
		},
		{
			name: "invalidation continuation with timeout",
			resp: &LiveChatResponse{
				ContinuationContents: ContinuationContents{
					LiveChatContinuation: LiveChatContinuation{
						Continuations: []Continuation{
							{
								InvalidationContinuationData: &InvalidationContinuationData{
									Continuation: "token",
									TimeoutDurationMillis: 5000,
								},
							},
						},
					},
				},
			},
			want: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.GetPollInterval(tt.resp)
			if got != tt.want {
				t.Errorf("GetPollInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantType    ErrorType
		wantFatal   bool
		wantTransient bool
	}{
		{
			name:        "nil error",
			err:         nil,
			wantType:    ErrorTypeTransient,
			wantFatal:   false,
			wantTransient: true,
		},
		{
			name: "401 unauthorized - fatal",
			err: &HTTPStatusError{StatusCode: 401},
			wantType:    ErrorTypeFatal,
			wantFatal:   true,
			wantTransient: false,
		},
		{
			name: "403 forbidden - fatal",
			err: &HTTPStatusError{StatusCode: 403},
			wantType:    ErrorTypeFatal,
			wantFatal:   true,
			wantTransient: false,
		},
		{
			name: "404 not found - fatal",
			err: &HTTPStatusError{StatusCode: 404},
			wantType:    ErrorTypeFatal,
			wantFatal:   true,
			wantTransient: false,
		},
		{
			name: "429 rate limit - transient",
			err: &HTTPStatusError{StatusCode: 429},
			wantType:    ErrorTypeTransient,
			wantFatal:   false,
			wantTransient: true,
		},
		{
			name: "500 server error - transient",
			err: &HTTPStatusError{StatusCode: 500},
			wantType:    ErrorTypeTransient,
			wantFatal:   false,
			wantTransient: true,
		},
		{
			name: "502 bad gateway - transient",
			err: &HTTPStatusError{StatusCode: 502},
			wantType:    ErrorTypeTransient,
			wantFatal:   false,
			wantTransient: true,
		},
		{
			name: "503 service unavailable - transient",
			err: &HTTPStatusError{StatusCode: 503},
			wantType:    ErrorTypeTransient,
			wantFatal:   false,
			wantTransient: true,
		},
		{
			name: "504 gateway timeout - transient",
			err: &HTTPStatusError{StatusCode: 504},
			wantType:    ErrorTypeTransient,
			wantFatal:   false,
			wantTransient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType := ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("ClassifyError() = %v, want %v", gotType, tt.wantType)
			}

			gotFatal := IsFatalError(tt.err)
			if gotFatal != tt.wantFatal {
				t.Errorf("IsFatalError() = %v, want %v", gotFatal, tt.wantFatal)
			}

			gotTransient := IsTransientError(tt.err)
			if gotTransient != tt.wantTransient {
				t.Errorf("IsTransientError() = %v, want %v", gotTransient, tt.wantTransient)
			}
		})
	}
}
