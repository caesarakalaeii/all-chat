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
