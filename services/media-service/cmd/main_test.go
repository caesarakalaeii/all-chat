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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// These tests pin the OTEL env wiring. The deployment manifest (caesar-deployment
// apps/workloads/all-chat/media-service-deployment.yaml) sets these variables; a
// typo'd name would leave them dead config advertising telemetry the service
// never emits — the defect that prompted this wiring in the first place.

func TestTracingConfig_DisabledByDefault(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "")

	cfg := tracingConfigFromEnv()

	assert.False(t, cfg.Enabled, "tracing must be off unless OTEL_ENABLED=true")
	assert.Equal(t, "media-service", cfg.ServiceName)
}

func TestTracingConfig_ReadsTheManifestOTELVars(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector.observability:4317")

	cfg := tracingConfigFromEnv()

	assert.True(t, cfg.Enabled, "OTEL_ENABLED=true must turn tracing on")
	assert.Equal(t, "otel-collector.observability:4317", cfg.OTLPEndpoint)
}
