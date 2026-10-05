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

// TestLoadConfig_Defaults: an unconfigured local run must come up with the
// documented defaults — port 8095 and the allchat dev database — rather than
// empty strings that only fail at connect time.
func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_HOST", "")
	t.Setenv("DATABASE_PORT", "")
	t.Setenv("DATABASE_NAME", "")
	t.Setenv("DATABASE_USER", "")

	cfg := loadConfig()
	assert.Equal(t, "8095", cfg.Port)
	assert.Equal(t, "localhost", cfg.DatabaseHost)
	assert.Equal(t, "5432", cfg.DatabasePort)
	assert.Equal(t, "allchat", cfg.DatabaseName)
	assert.Equal(t, "allchat", cfg.DatabaseUser)
}

// TestLoadConfig_EnvOverrides: the compose/manifest wiring must actually reach
// the service — a typo'd variable name would silently fall back to localhost.
func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "1234")
	t.Setenv("DATABASE_HOST", "postgres")
	t.Setenv("DATABASE_PORT", "5433")
	t.Setenv("DATABASE_NAME", "other")
	t.Setenv("DATABASE_USER", "someone")

	cfg := loadConfig()
	assert.Equal(t, "1234", cfg.Port)
	assert.Equal(t, "postgres", cfg.DatabaseHost)
	assert.Equal(t, "5433", cfg.DatabasePort)
	assert.Equal(t, "other", cfg.DatabaseName)
	assert.Equal(t, "someone", cfg.DatabaseUser)
}

// TestTracingConfig_DisabledByDefault: telemetry must be opt-in, so a local
// run without an OTLP collector is clean.
func TestTracingConfig_DisabledByDefault(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "")

	cfg := tracingConfigFromEnv()
	assert.False(t, cfg.Enabled)
	assert.Equal(t, "alert-processor", cfg.ServiceName)
}

// TestTracingConfig_ReadsTheManifestOTELVars: the deployment manifest's OTEL
// block must reach tracing.Config unchanged.
func TestTracingConfig_ReadsTheManifestOTELVars(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("ENVIRONMENT", "production")

	cfg := tracingConfigFromEnv()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "otel-collector:4317", cfg.OTLPEndpoint)
	assert.Equal(t, "production", cfg.Environment)
}