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

func TestNewBusinessMetrics_UserRegistrationsNotNil(t *testing.T) {
	m := newTestBusinessMetrics()
	if m.UserRegistrations == nil {
		t.Fatal("expected UserRegistrations to be non-nil")
	}
}

func TestRecordUserRegistration_Twitch(t *testing.T) {
	m := newTestBusinessMetrics()
	m.RecordUserRegistration("twitch")

	expected := `
# HELP allchat_user_registrations_total Total new user registrations by auth platform
# TYPE allchat_user_registrations_total counter
allchat_user_registrations_total{platform="twitch"} 1
`
	if err := testutil.CollectAndCompare(m.UserRegistrations, strings.NewReader(expected)); err != nil {
		t.Fatalf("unexpected metric state: %v", err)
	}
}

func TestRecordUserRegistration_YouTube(t *testing.T) {
	m := newTestBusinessMetrics()
	m.RecordUserRegistration("youtube")

	expected := `
# HELP allchat_user_registrations_total Total new user registrations by auth platform
# TYPE allchat_user_registrations_total counter
allchat_user_registrations_total{platform="youtube"} 1
`
	if err := testutil.CollectAndCompare(m.UserRegistrations, strings.NewReader(expected)); err != nil {
		t.Fatalf("unexpected metric state: %v", err)
	}
}
