package tracing

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPClientConfig holds configuration for instrumented HTTP client
type HTTPClientConfig struct {
	Timeout time.Duration
	// Add other http.Client fields as needed
}

// NewInstrumentedClient creates an HTTP client with OpenTelemetry instrumentation
// If tracing is disabled, returns a standard http.Client without instrumentation
func NewInstrumentedClient(enabled bool, serviceName string, cfg *HTTPClientConfig) *http.Client {
	if cfg == nil {
		cfg = &HTTPClientConfig{
			Timeout: 30 * time.Second,
		}
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	if !enabled {
		return client
	}

	// Wrap transport with OpenTelemetry instrumentation
	client.Transport = otelhttp.NewTransport(
		http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return serviceName + " " + operation + " " + r.Method + " " + r.URL.Path
		}),
	)

	return client
}

// NewInstrumentedClientWithTransport creates an HTTP client with a custom base transport
// and OpenTelemetry instrumentation. Useful for OAuth2 clients or custom transports.
func NewInstrumentedClientWithTransport(enabled bool, serviceName string, baseTransport http.RoundTripper, cfg *HTTPClientConfig) *http.Client {
	if cfg == nil {
		cfg = &HTTPClientConfig{
			Timeout: 30 * time.Second,
		}
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	if !enabled {
		client.Transport = baseTransport
		return client
	}

	// Wrap the provided transport with OpenTelemetry instrumentation
	client.Transport = otelhttp.NewTransport(
		baseTransport,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return serviceName + " " + operation + " " + r.Method + " " + r.URL.Path
		}),
	)

	return client
}
