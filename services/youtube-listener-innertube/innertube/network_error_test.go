package innertube

import (
	"errors"
	"testing"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
)

// TestClassifyNetworkError tests the classifyNetworkError helper function
// This test is standalone and doesn't depend on the rest of the innertube package
func TestClassifyNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "DNS error",
			err:      errors.New("lookup youtube.com: no such host"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "DNS error - alternate format",
			err:      errors.New("dns lookup failed"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "Connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "Connection reset",
			err:      errors.New("read tcp: connection reset by peer"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "Broken pipe",
			err:      errors.New("write tcp: broken pipe"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "Timeout",
			err:      errors.New("context deadline exceeded"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "Timeout - alternate format",
			err:      errors.New("i/o timeout"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "TLS error",
			err:      errors.New("tls: handshake failure"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "TLS certificate error",
			err:      errors.New("x509: certificate has expired"),
			expected: metrics.ErrorTypeNetwork,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "Generic network error",
			err:      errors.New("unknown network error"),
			expected: metrics.ErrorTypeNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyNetworkError(tt.err)
			if got != tt.expected {
				t.Errorf("classifyNetworkError() = %v, want %v", got, tt.expected)
			}
		})
	}
}
