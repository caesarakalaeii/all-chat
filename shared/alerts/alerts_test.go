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

package alerts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryKnownEventTypeHasAUsableDefault(t *testing.T) {
	require.NotEmpty(t, EventTypes())
	for _, et := range EventTypes() {
		p, ok := DefaultProfile(et)
		require.True(t, ok, et)
		assert.Equal(t, et, p.EventType)
		assert.NotEmpty(t, p.TextTemplate, et)
		assert.Contains(t, p.TextTemplate, "{username}", et)
		assert.Equal(t, 5000, p.DurationMs, et)
		assert.Equal(t, 100, p.Volume, et)
		assert.True(t, p.Enabled, et)
		assert.Empty(t, p.SoundURL, "defaults carry no media")
		assert.Empty(t, p.ImageURL, "defaults carry no media")
		require.NoError(t, p.Validate(), et)
	}
}

// The set must use the raw event_type names the listeners publish and the
// alert-processor forwards, not invented ones.
func TestEventTypesMatchTheNamesThePipelineEmits(t *testing.T) {
	for _, et := range []string{
		"subscription", "resubscription", "gift_subscription", "mystery_gift", "bits", "raid",
		"follow", "channel_points", "charity_donation", "super_chat", "super_sticker",
		"new_sponsor", "member_milestone", "membership_gift", "subscriber", "gift", "share",
		"tiktok_subscription", "power_up", "gift_received",
	} {
		assert.True(t, IsKnownEventType(et), et)
	}
	for _, et := range []string{"", "chat", "message_deletion", "like_aggregate", "Bits"} {
		assert.False(t, IsKnownEventType(et), et)
	}
}

func TestProfileValidate(t *testing.T) {
	base, _ := DefaultProfile("bits")
	cases := map[string]func(p *Profile){
		"unknown event type":   func(p *Profile) { p.EventType = "webcam" },
		"volume above 100":     func(p *Profile) { p.Volume = 101 },
		"negative volume":      func(p *Profile) { p.Volume = -1 },
		"zero duration":        func(p *Profile) { p.DurationMs = 0 },
		"duration over 60s":    func(p *Profile) { p.DurationMs = 60001 },
		"javascript sound url": func(p *Profile) { p.SoundURL = "javascript:alert(1)" },
		"relative image url":   func(p *Profile) { p.ImageURL = "/x.png" },
		"template too long":    func(p *Profile) { p.TextTemplate = string(make([]byte, 501)) },
		"custom html too long": func(p *Profile) { p.CustomHTML = string(make([]byte, MaxCustomCodeLen+1)) },
		"custom css too long":  func(p *Profile) { p.CustomCSS = string(make([]byte, MaxCustomCodeLen+1)) },
		"custom js too long":   func(p *Profile) { p.CustomJS = string(make([]byte, MaxCustomCodeLen+1)) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := base
			mutate(&p)
			assert.Error(t, p.Validate())
		})
	}
	ok := base
	ok.SoundURL = "https://media.allch.at/u/1/ding.mp3"
	ok.ImageURL = "https://media.allch.at/u/1/party.gif"
	ok.CustomHTML = string(make([]byte, MaxCustomCodeLen))
	ok.CustomCSS = "p { color: red }"
	ok.CustomJS = "onAlert(() => {})"
	assert.NoError(t, ok.Validate())
}

func TestRenderTextSubstitutesPlaceholders(t *testing.T) {
	a := Alert{
		EventType: "bits",
		Message:   "take my bits",
		User:      User{Name: "SomeChatter"},
		EventData: &EventData{Value: &Value{Amount: 100, Currency: "bits", DisplayText: "100 bits"}},
	}
	assert.Equal(t, "SomeChatter cheered 100 bits (100): take my bits",
		RenderText("{username} cheered {amount} ({count}): {message}", a))
}

func TestRenderTextKeepsTheLiteralTokenWhenAFieldIsMissing(t *testing.T) {
	assert.Equal(t, "{username} cheered {amount} ({count}): {message} {unknown}",
		RenderText("{username} cheered {amount} ({count}): {message} {unknown}", Alert{}))

	// A value with no display text still has a count.
	a := Alert{EventData: &EventData{Value: &Value{Amount: 5}}}
	assert.Equal(t, "{amount} 5", RenderText("{amount} {count}", a))
}

func TestResolveAttachesTheRenderedText(t *testing.T) {
	p, _ := DefaultProfile("raid")
	a := Alert{EventType: "raid", User: User{Name: "Raider"}, EventData: &EventData{Value: &Value{Amount: 42, DisplayText: "42 viewers"}}}
	r := Resolve(p, a)
	assert.Equal(t, "Raider is raiding with 42 viewers!", r.Text)
	assert.Equal(t, p, r.Profile)

	raw, err := json.Marshal(r)
	require.NoError(t, err)
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keys))
	for _, k := range []string{"event_type", "text", "text_template", "sound_url", "image_url", "duration_ms", "volume", "enabled", "sort_after"} {
		assert.Contains(t, keys, k)
	}
}

func TestTestAlertMatchesThePipelineEnvelope(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a, err := TestAlert("overlay-1", "super_chat", now)
	require.NoError(t, err)
	b, err := TestAlert("overlay-1", "super_chat", now)
	require.NoError(t, err)
	assert.NotEqual(t, a.AlertID, b.AlertID, "each test press is a fresh alert the client must not dedup away")

	assert.Equal(t, "overlay-1", a.OverlayID)
	assert.Equal(t, "youtube", a.Platform)
	assert.Equal(t, "super_chat", a.EventType)
	assert.Equal(t, now, a.OccurredAt)
	require.NotNil(t, a.EventData)
	assert.Equal(t, "super_chat", a.EventData.Type)
	require.NotNil(t, a.EventData.Value)
	assert.NotEmpty(t, a.EventData.Value.DisplayText)
	assert.Equal(t, true, a.EventData.Metadata["test"])
	assert.NotEmpty(t, a.User.Name)

	raw, err := json.Marshal(a)
	require.NoError(t, err)
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keys))
	assert.ElementsMatch(t,
		[]string{"alert_id", "overlay_id", "platform", "event_type", "event_data", "user", "message", "occurred_at"},
		keysOf(keys), "same keys as services/alert-processor/models.Alert")

	_, err = TestAlert("overlay-1", "webcam", now)
	assert.Error(t, err)
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestAllEventsIsAStorableProfileButNotAnEventType(t *testing.T) {
	assert.True(t, IsKnownEventType(AllEvents), "the base style is saved like any profile")
	assert.NotContains(t, EventTypes(), AllEvents, "nothing is ever published as the base style")

	base, ok := DefaultProfile(AllEvents)
	require.True(t, ok)
	assert.Equal(t, Profile{EventType: AllEvents, DurationMs: 5000, Volume: 100, Enabled: true}, base)
	require.NoError(t, base.Validate())

	_, err := TestAlert("overlay-1", AllEvents, time.Now())
	assert.Error(t, err, "a test alert needs a real event type")
}

func TestApplyBaseStylesEveryUnsavedEventType(t *testing.T) {
	base, _ := DefaultProfile(AllEvents)
	base.ImageURL = "https://static.klipy.com/party.gif"
	base.SoundURL = "https://media.allch.at/ding.mp3"
	base.DurationMs = 8000
	base.Volume = 40
	base.CustomCSS = "p { color: red }"

	def, _ := DefaultProfile("raid")
	got := ApplyBase(def, false, base)

	assert.Equal(t, "raid", got.EventType)
	assert.Equal(t, def.TextTemplate, got.TextTemplate, "the text stays per event type")
	assert.Equal(t, base.ImageURL, got.ImageURL)
	assert.Equal(t, base.SoundURL, got.SoundURL)
	assert.Equal(t, 8000, got.DurationMs)
	assert.Equal(t, 40, got.Volume)
	assert.Equal(t, "p { color: red }", got.CustomCSS)
	assert.True(t, got.Enabled)
}

func TestApplyBaseFillsOnlyWhatASavedEventTypeLeavesEmpty(t *testing.T) {
	base, _ := DefaultProfile(AllEvents)
	base.ImageURL = "https://static.klipy.com/party.gif"
	base.SoundURL = "https://media.allch.at/ding.mp3"
	base.DurationMs = 8000
	base.CustomHTML = "<p data-alert-text></p>"

	saved, _ := DefaultProfile("bits")
	saved.TextTemplate = "{username} threw {amount}"
	saved.ImageURL = "https://static.klipy.com/bits.gif"
	saved.DurationMs = 3000
	saved.CustomJS = "onAlert(() => {})"

	got := ApplyBase(saved, true, base)
	assert.Equal(t, "{username} threw {amount}", got.TextTemplate)
	assert.Equal(t, saved.ImageURL, got.ImageURL, "the event's own GIF wins")
	assert.Equal(t, base.SoundURL, got.SoundURL, "an empty sound falls back to the base")
	assert.Equal(t, 3000, got.DurationMs, "a saved event keeps its own timing")
	assert.Equal(t, "onAlert(() => {})", got.CustomJS)
	assert.Empty(t, got.CustomHTML, "custom code is taken as a unit, never mixed with the base's")
}

func TestApplyBaseSwitchedOffSilencesEveryEventType(t *testing.T) {
	base, _ := DefaultProfile(AllEvents)
	base.Enabled = false
	saved, _ := DefaultProfile("follow")
	assert.False(t, ApplyBase(saved, true, base).Enabled)

	def, _ := DefaultProfile("follow")
	assert.False(t, ApplyBase(def, false, base).Enabled)

	base.Enabled = true
	saved.Enabled = false
	assert.False(t, ApplyBase(saved, true, base).Enabled, "an event switched off stays off")
}

func TestApplyBaseWithTheDefaultBaseChangesNothing(t *testing.T) {
	base, _ := DefaultProfile(AllEvents)
	for _, et := range EventTypes() {
		def, _ := DefaultProfile(et)
		assert.Equal(t, def, ApplyBase(def, false, base), et)
	}
}
