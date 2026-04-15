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
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestBusinessMetrics creates a BusinessMetrics with a fresh, isolated
// prometheus registry so tests don't conflict with promauto's default registry.
func newTestBusinessMetrics() *BusinessMetrics {
	reg := prometheus.NewRegistry()
	return newBusinessMetricsWithRegistry(reg)
}

func TestNewBusinessMetrics_FieldsNotNil(t *testing.T) {
	m := newTestBusinessMetrics()
	if m.UserRegistrations == nil {
		t.Fatal("expected UserRegistrations to be non-nil")
	}
	if m.ViewerRegistrations == nil {
		t.Fatal("expected ViewerRegistrations to be non-nil")
	}
	if m.TotalUsersByPlatform == nil {
		t.Fatal("expected TotalUsersByPlatform to be non-nil")
	}
}

func TestPreInitialisation_KnownPlatformsPresent(t *testing.T) {
	m := newTestBusinessMetrics()

	// All known platforms must be pre-initialised at zero, i.e. visible in /metrics
	// even before any registration occurs.
	var sb strings.Builder
	sb.WriteString("# HELP allchat_user_registrations_total Total new streamer registrations by auth platform\n")
	sb.WriteString("# TYPE allchat_user_registrations_total counter\n")
	for _, p := range knownPlatforms {
		sb.WriteString("allchat_user_registrations_total{platform=\"" + p + "\"} 0\n")
	}
	if err := testutil.CollectAndCompare(m.UserRegistrations, strings.NewReader(sb.String())); err != nil {
		t.Fatalf("pre-initialisation check failed: %v", err)
	}
}

func TestRecordUserRegistration_Twitch(t *testing.T) {
	m := newTestBusinessMetrics()
	m.RecordUserRegistration("twitch")

	expected := `
# HELP allchat_user_registrations_total Total new streamer registrations by auth platform
# TYPE allchat_user_registrations_total counter
allchat_user_registrations_total{platform="kick"} 0
allchat_user_registrations_total{platform="twitch"} 1
allchat_user_registrations_total{platform="youtube"} 0
`
	if err := testutil.CollectAndCompare(m.UserRegistrations, strings.NewReader(expected)); err != nil {
		t.Fatalf("unexpected metric state: %v", err)
	}
}

func TestRecordUserRegistration_YouTube(t *testing.T) {
	m := newTestBusinessMetrics()
	m.RecordUserRegistration("youtube")

	expected := `
# HELP allchat_user_registrations_total Total new streamer registrations by auth platform
# TYPE allchat_user_registrations_total counter
allchat_user_registrations_total{platform="kick"} 0
allchat_user_registrations_total{platform="twitch"} 0
allchat_user_registrations_total{platform="youtube"} 1
`
	if err := testutil.CollectAndCompare(m.UserRegistrations, strings.NewReader(expected)); err != nil {
		t.Fatalf("unexpected metric state: %v", err)
	}
}

func TestRecordViewerRegistration_Twitch(t *testing.T) {
	m := newTestBusinessMetrics()
	m.RecordViewerRegistration("twitch")

	expected := `
# HELP allchat_viewer_registrations_total Total new viewer registrations by auth platform
# TYPE allchat_viewer_registrations_total counter
allchat_viewer_registrations_total{platform="kick"} 0
allchat_viewer_registrations_total{platform="twitch"} 1
allchat_viewer_registrations_total{platform="youtube"} 0
`
	if err := testutil.CollectAndCompare(m.ViewerRegistrations, strings.NewReader(expected)); err != nil {
		t.Fatalf("unexpected metric state: %v", err)
	}
}

func TestInitTotalUsersByPlatform(t *testing.T) {
	m := newTestBusinessMetrics()
	m.InitTotalUsersByPlatform(map[string]int64{
		"twitch":  42,
		"youtube": 7,
	})

	expected := `
# HELP allchat_total_users_by_platform Total registered streamers per auth platform, seeded from the database at startup
# TYPE allchat_total_users_by_platform gauge
allchat_total_users_by_platform{platform="kick"} 0
allchat_total_users_by_platform{platform="twitch"} 42
allchat_total_users_by_platform{platform="youtube"} 7
`
	if err := testutil.CollectAndCompare(m.TotalUsersByPlatform, strings.NewReader(expected)); err != nil {
		t.Fatalf("unexpected metric state: %v", err)
	}
}
