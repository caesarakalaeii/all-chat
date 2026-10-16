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
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bitsAlert = `{"alert_id":"a-1","overlay_id":"o-1","platform":"twitch","event_type":"bits",
"event_data":{"type":"bits","tier":"high","value":{"amount":100,"currency":"bits","display_text":"100 bits"},"duration":8,"is_update":false},
"user":{"id":"u-1","name":"SomeChatter","avatar_url":""},"message":"take my bits","occurred_at":"2026-10-16T12:00:00Z"}`

var frameTime = time.Date(2026, 10, 16, 12, 0, 1, 0, time.UTC)

func noProfile(context.Context, string, string) (alerts.Profile, bool, error) {
	return alerts.Profile{}, false, nil
}

func decodeFrame(t *testing.T, frame []byte) (string, map[string]json.RawMessage) {
	t.Helper()
	var env struct {
		Type string                     `json:"type"`
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frame, &env), string(frame))
	return env.Type, env.Data
}

func TestBuildAlertFrame_ForwardsTheAlertWithTheDefaultProfile(t *testing.T) {
	frame, err := buildAlertFrame(context.Background(), "o-1", []byte(bitsAlert), noProfile, frameTime)
	require.NoError(t, err)
	require.NotNil(t, frame)

	typ, data := decodeFrame(t, frame)
	assert.Equal(t, "alert", typ)
	for _, k := range []string{"alert_id", "overlay_id", "platform", "event_type", "event_data", "user", "message", "occurred_at"} {
		assert.Contains(t, data, k, "the alert envelope is forwarded unchanged")
	}
	var profile alerts.ResolvedProfile
	require.NoError(t, json.Unmarshal(data["profile"], &profile))
	def, _ := alerts.DefaultProfile("bits")
	assert.Equal(t, def, profile.Profile)
	assert.Equal(t, "SomeChatter cheered 100 bits!", profile.Text)
}

func TestBuildAlertFrame_UsesTheSavedProfile(t *testing.T) {
	saved, _ := alerts.DefaultProfile("bits")
	saved.TextTemplate = "{username}: {message} ({count} {nope})"
	saved.SoundURL = "https://media.allch.at/u/ding.mp3"
	saved.CustomHTML = "<b data-alert-text></b>"
	saved.CustomCSS = "b { color: gold }"
	saved.CustomJS = "onAlert(function () {})"
	lookup := func(_ context.Context, overlayID, eventType string) (alerts.Profile, bool, error) {
		assert.Equal(t, "o-1", overlayID)
		assert.Equal(t, "bits", eventType)
		return saved, true, nil
	}

	frame, err := buildAlertFrame(context.Background(), "o-1", []byte(bitsAlert), lookup, frameTime)
	require.NoError(t, err)
	_, data := decodeFrame(t, frame)
	var profile alerts.ResolvedProfile
	require.NoError(t, json.Unmarshal(data["profile"], &profile))
	assert.Equal(t, "SomeChatter: take my bits (100 {nope})", profile.Text)
	assert.Equal(t, saved.SoundURL, profile.SoundURL)
	// Custom code reaches the overlay in the frame: the renderer needs no second fetch.
	assert.Equal(t, saved.CustomHTML, profile.CustomHTML)
	assert.Equal(t, saved.CustomCSS, profile.CustomCSS)
	assert.Equal(t, saved.CustomJS, profile.CustomJS)
}

func TestBuildAlertFrame_DropsDisabledProfiles(t *testing.T) {
	off, _ := alerts.DefaultProfile("bits")
	off.Enabled = false
	frame, err := buildAlertFrame(context.Background(), "o-1", []byte(bitsAlert),
		func(context.Context, string, string) (alerts.Profile, bool, error) { return off, true, nil }, frameTime)
	require.NoError(t, err)
	assert.Nil(t, frame, "a streamer who switched an event type off must not see it")
}

func TestBuildAlertFrame_FallsBackToTheDefaultWhenTheLookupFails(t *testing.T) {
	frame, err := buildAlertFrame(context.Background(), "o-1", []byte(bitsAlert),
		func(context.Context, string, string) (alerts.Profile, bool, error) {
			return alerts.Profile{}, false, errors.New("db down")
		}, frameTime)
	require.Error(t, err, "the caller logs the lookup failure")
	require.NotNil(t, frame, "but the alert still reaches the overlay")
	_, data := decodeFrame(t, frame)
	assert.Contains(t, data, "profile")
}

func TestBuildAlertFrame_EventTypesWithoutProfilesCarryNone(t *testing.T) {
	raw := `{"alert_id":"a-2","overlay_id":"o-1","platform":"tiktok","event_type":"like_aggregate","event_data":null,"user":{"id":"","name":"x","avatar_url":""},"occurred_at":"2026-10-16T12:00:00Z"}`
	frame, err := buildAlertFrame(context.Background(), "o-1", []byte(raw), noProfile, frameTime)
	require.NoError(t, err)
	_, data := decodeFrame(t, frame)
	assert.NotContains(t, data, "profile")
	assert.Contains(t, data, "alert_id")
}

func TestBuildAlertFrame_RejectsGarbage(t *testing.T) {
	frame, err := buildAlertFrame(context.Background(), "o-1", []byte("not json"), noProfile, frameTime)
	assert.Error(t, err)
	assert.Nil(t, frame)
}
