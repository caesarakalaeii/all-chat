package tracing

import (
	"os"
	"strconv"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// AlwaysSampleErrorsSampler wraps base sampler and always samples error traces
type AlwaysSampleErrorsSampler struct {
	delegate sdktrace.Sampler
}

func (s *AlwaysSampleErrorsSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	// Check span status attributes for errors
	for _, attr := range p.Attributes {
		if attr.Key == "error" && attr.Value.AsBool() {
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: trace.SpanContextFromContext(p.ParentContext).TraceState(),
			}
		}
	}
	// Delegate to base sampler for non-errors
	return s.delegate.ShouldSample(p)
}

func (s *AlwaysSampleErrorsSampler) Description() string {
	return "AlwaysSampleErrorsSampler{" + s.delegate.Description() + "}"
}

// createConfigurableSampler reads OTEL_SAMPLING_RATE and creates sampler
func createConfigurableSampler() sdktrace.Sampler {
	samplingRateStr := os.Getenv("OTEL_SAMPLING_RATE")
	samplingRate := 1.0 // Default 100% for initial weeks
	if samplingRateStr != "" {
		if rate, err := strconv.ParseFloat(samplingRateStr, 64); err == nil {
			if rate >= 0.0 && rate <= 1.0 {
				samplingRate = rate
			}
		}
	}

	baseSampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(samplingRate),
	)

	return &AlwaysSampleErrorsSampler{
		delegate: baseSampler,
	}
}
