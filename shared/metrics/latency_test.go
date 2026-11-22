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

package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// The gateway reports ingest-to-overlay lag through this histogram, so it has to
// exist on the metrics struct and accept an observation labelled by service and
// platform.
func TestPipelineEndToEndLatencyRegistered(t *testing.T) {
	m := NewGatewayMetricsForTest()
	if m.PipelineEndToEndLatency == nil {
		t.Fatal("PipelineEndToEndLatency is nil")
	}

	m.RecordPipelineEndToEndLatency("api-gateway", "twitch", 250*time.Millisecond)

	if got := testutil.CollectAndCount(m.PipelineEndToEndLatency); got != 1 {
		t.Errorf("collected %d series after one observation, want 1", got)
	}
}

// gateway_message_delivery_latency_seconds was registered but never observed by
// any caller, so it had no data at all. The recorder is the missing surface.
func TestRecordMessageDeliveryLatency(t *testing.T) {
	m := NewGatewayMetricsForTest()

	m.RecordMessageDeliveryLatency("api-gateway", 5*time.Millisecond)

	if got := testutil.CollectAndCount(m.MessageDeliveryLatency); got != 1 {
		t.Errorf("collected %d series after one observation, want 1", got)
	}
}

// Enrichment p95 saturated the old top finite bucket of 0.1s, which hid the real
// tail: every slow stage landed in +Inf and the quantile read exactly 0.1. The
// buckets must reach past 100ms for the tail to be visible at all.
func TestStageDurationBucketsAbove100ms(t *testing.T) {
	var above []float64
	for _, upperBound := range stageDurationBuckets {
		if upperBound > 0.1 {
			above = append(above, upperBound)
		}
	}
	if len(above) == 0 {
		t.Errorf("stage duration buckets top out at 0.1s (%v); the enrichment tail is clipped", stageDurationBuckets)
	}
}
