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

// Package alerts holds the alert-profile model shared by overlay-manager (which
// stores profiles and publishes test alerts) and the api-gateway (which
// resolves a profile onto every alert it forwards): the known event types,
// their built-in default profiles, placeholder substitution, and the alert
// envelope the alert-processor publishes to overlay:{id}:alerts (ADR-0068,
// ADR-0070).
package alerts

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Alert is the envelope on overlay:{overlay_id}:alerts. It mirrors
// services/alert-processor/models.Alert key for key; the alert-processor keeps
// its own copy because its event_data is the message-processor's EventInfo.
// ImageURL is the one extra: only chat command alerts set it.
type Alert struct {
	AlertID    string     `json:"alert_id"`
	OverlayID  string     `json:"overlay_id"`
	Platform   string     `json:"platform"`
	EventType  string     `json:"event_type"`
	EventData  *EventData `json:"event_data"`
	User       User       `json:"user"`
	Message    string     `json:"message"`
	OccurredAt time.Time  `json:"occurred_at"`
	// ImageURL replaces the profile's image for this one alert: a chat command's
	// own image (ADR-0080). engagement-service publishes those directly.
	ImageURL string `json:"image_url,omitempty"`
}

// ChatCommandEvent is the event type of a chat command that shows an image
// (ADR-0080). Its {message} is the command's reply, or what the chatter typed
// when the command has none; {amount} is the trigger and {count} its uses.
const ChatCommandEvent = "chat_command"

// EventData is the message-processor's EventInfo as it appears on the wire.
type EventData struct {
	Type     string         `json:"type"`
	Tier     string         `json:"tier"`
	Value    *Value         `json:"value,omitempty"`
	Duration int            `json:"duration"`
	IsUpdate bool           `json:"is_update"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Value is the message-processor's EventValue.
type Value struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	DisplayText string  `json:"display_text"`
}

// User is the event sender.
type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// Profile is how an alerts overlay presents one event type (alert_profiles row).
type Profile struct {
	EventType    string `json:"event_type"`
	TextTemplate string `json:"text_template"`
	SoundURL     string `json:"sound_url"`
	ImageURL     string `json:"image_url"`
	DurationMs   int    `json:"duration_ms"`
	Volume       int    `json:"volume"`
	Enabled      bool   `json:"enabled"`
	SortAfter    int    `json:"sort_after"`
	// Custom code (premium, migration 109, ADR-0071): when any is set, the
	// alert box renders it in a sandboxed iframe instead of the built-in
	// layout. Never validated beyond size: the sandbox is the isolation.
	CustomHTML string `json:"custom_html"`
	CustomCSS  string `json:"custom_css"`
	CustomJS   string `json:"custom_js"`
}

// ResolvedProfile is a profile with its template rendered for one alert.
type ResolvedProfile struct {
	Profile
	Text string `json:"text"`
}

const maxTemplateLen = 500

// AllEvents is the pseudo event type of an overlay's base style (ADR-0075):
// the media, timing and custom code every event type uses unless it saves its
// own. It is stored like a profile but never published as an event.
const AllEvents = "all"

// MaxCustomCodeLen caps each custom code field: it travels in every alert
// frame and is stored per event type.
const MaxCustomCodeLen = 64 * 1024

type eventDef struct {
	eventType, platform, template string
	sample                        Value
}

// eventDefs is every event type a profile can be set for, using the raw
// event_type names the listeners publish on chat:raw. Event types the
// alert-processor forwards but that are not listed (like_aggregate,
// watch_streak, ...) still reach the overlay, just without a profile.
var eventDefs = []eventDef{
	{"subscription", "twitch", "{username} just subscribed!", Value{1, "months", "Tier 1"}},
	{"resubscription", "twitch", "{username} resubscribed for {count} months!", Value{12, "months", "Tier 1 - 12 months"}},
	{"gift_subscription", "twitch", "{username} gifted a sub!", Value{1, "gift", "Gifted Tier 1 sub to a viewer"}},
	{"mystery_gift", "twitch", "{username} gifted {count} subs!", Value{5, "gifts", "5 gift subs"}},
	{"bits", "twitch", "{username} cheered {amount}!", Value{100, "bits", "100 bits"}},
	{"raid", "twitch", "{username} is raiding with {count} viewers!", Value{42, "viewers", "42 viewers"}},
	{"follow", "twitch", "{username} just followed!", Value{1, "follow", "New follower"}},
	{"channel_points", "twitch", "{username} redeemed {amount}!", Value{500, "points", "Hydrate (500 points)"}},
	{"charity_donation", "twitch", "{username} donated {amount} to charity!", Value{10, "USD", "10.00 USD"}},
	{"super_chat", "youtube", "{username} sent a Super Chat of {amount}!", Value{4990000, "USD", "$4.99"}},
	{"super_sticker", "youtube", "{username} sent a Super Sticker of {amount}!", Value{1990000, "USD", "$1.99"}},
	{"new_sponsor", "youtube", "{username} became a member!", Value{1, "membership", "New member"}},
	{"member_milestone", "youtube", "{username} has been a member for {count} months!", Value{6, "months", "6 month milestone"}},
	{"membership_gift", "youtube", "{username} gifted {count} memberships!", Value{5, "gifts", "5 gift memberships"}},
	{"subscriber", "youtube", "{username} just subscribed!", Value{1, "subscribers", "New subscriber"}},
	{"gift", "tiktok", "{username} sent {amount}!", Value{1, "diamonds", "Rose (1 diamonds)"}},
	{"share", "tiktok", "{username} shared the stream!", Value{1, "share", "Shared stream"}},
	{"tiktok_subscription", "tiktok", "{username} just subscribed!", Value{1, "months", "Subscribed"}},
	{"power_up", "twitch", "{username} used {amount}!", Value{50, "bits", "On-Screen Celebration"}},
	{"gift_received", "youtube", "{username} received a gift membership!", Value{1, "gift", "Received gift membership"}},
	// Fired from any platform's chat; the platform here is only the test alert's.
	{"chat_command", "twitch", "{username} used {amount}", Value{1, "uses", "!hype"}},
}

func lookup(eventType string) (eventDef, bool) {
	for _, d := range eventDefs {
		if d.eventType == eventType {
			return d, true
		}
	}
	return eventDef{}, false
}

// EventTypes lists every event type a profile can be set for, in display order.
func EventTypes() []string {
	out := make([]string, len(eventDefs))
	for i, d := range eventDefs {
		out[i] = d.eventType
	}
	return out
}

// IsKnownEventType reports whether eventType can carry a profile: a real
// event type or the AllEvents base style.
func IsKnownEventType(eventType string) bool {
	_, ok := lookup(eventType)
	return ok || eventType == AllEvents
}

// DefaultProfile is the built-in profile an alerts overlay uses for eventType
// until the streamer saves one: a text template and no media. The AllEvents
// default has no template either: the text is always per event type.
func DefaultProfile(eventType string) (Profile, bool) {
	if eventType == AllEvents {
		return Profile{EventType: AllEvents, DurationMs: 5000, Volume: 100, Enabled: true}, true
	}
	d, ok := lookup(eventType)
	if !ok {
		return Profile{}, false
	}
	return Profile{EventType: d.eventType, TextTemplate: d.template, DurationMs: 5000, Volume: 100, Enabled: true}, true
}

// ApplyBase layers an overlay's AllEvents base style under p, one event type's
// profile (ADR-0075). An unsaved p is the built-in default, which only has a
// text template, so it takes the base's media, timing and custom code. A saved
// p keeps its own timing and fills only an empty image or sound from the base;
// custom code moves as a unit, since mixing one profile's HTML with another's
// JS breaks both. A base switched off silences every event type.
func ApplyBase(p Profile, saved bool, base Profile) Profile {
	if !saved {
		p.DurationMs, p.Volume = base.DurationMs, base.Volume
	}
	if p.ImageURL == "" {
		p.ImageURL = base.ImageURL
	}
	if p.SoundURL == "" {
		p.SoundURL = base.SoundURL
	}
	if p.CustomHTML == "" && p.CustomCSS == "" && p.CustomJS == "" {
		p.CustomHTML, p.CustomCSS, p.CustomJS = base.CustomHTML, base.CustomCSS, base.CustomJS
	}
	p.Enabled = p.Enabled && base.Enabled
	return p
}

// Validate rejects a profile the overlay could not safely render.
func (p Profile) Validate() error {
	if !IsKnownEventType(p.EventType) {
		return fmt.Errorf("unknown event_type %q", p.EventType)
	}
	if len(p.TextTemplate) > maxTemplateLen {
		return fmt.Errorf("%s: text_template is longer than %d characters", p.EventType, maxTemplateLen)
	}
	for field, code := range map[string]string{"custom_html": p.CustomHTML, "custom_css": p.CustomCSS, "custom_js": p.CustomJS} {
		if len(code) > MaxCustomCodeLen {
			return fmt.Errorf("%s: %s is longer than %d bytes", p.EventType, field, MaxCustomCodeLen)
		}
	}
	if p.DurationMs < 1 || p.DurationMs > 60000 {
		return fmt.Errorf("%s: duration_ms must be between 1 and 60000", p.EventType)
	}
	if p.Volume < 0 || p.Volume > 100 {
		return fmt.Errorf("%s: volume must be between 0 and 100", p.EventType)
	}
	for field, raw := range map[string]string{"sound_url": p.SoundURL, "image_url": p.ImageURL} {
		if raw == "" {
			continue
		}
		// The overlay loads these into OBS with no further checks.
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("%s: %s must be an absolute http(s) URL", p.EventType, field)
		}
	}
	return nil
}

// RenderText substitutes {username}, {amount}, {count} and {message} from the
// alert. A placeholder whose field is missing stays as its literal token, so a
// streamer sees which part of the template had nothing to fill it.
func RenderText(template string, a Alert) string {
	var pairs []string
	if a.User.Name != "" {
		pairs = append(pairs, "{username}", a.User.Name)
	}
	if a.EventData != nil && a.EventData.Value != nil {
		if a.EventData.Value.DisplayText != "" {
			pairs = append(pairs, "{amount}", a.EventData.Value.DisplayText)
		}
		pairs = append(pairs, "{count}", strconv.FormatFloat(a.EventData.Value.Amount, 'f', -1, 64))
	}
	if a.Message != "" {
		pairs = append(pairs, "{message}", a.Message)
	}
	if len(pairs) == 0 {
		return template
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

// Resolve renders p for a.
func Resolve(p Profile, a Alert) ResolvedProfile {
	if a.ImageURL != "" {
		p.ImageURL = a.ImageURL
	}
	return ResolvedProfile{Profile: p, Text: RenderText(p.TextTemplate, a)}
}

// TestAlert builds a synthetic alert for the dashboard's test button: the
// pipeline's envelope with a sample value, a fresh alert_id per call (clients
// dedup on alert_id) and metadata.test=true.
func TestAlert(overlayID, eventType string, now time.Time) (Alert, error) {
	d, ok := lookup(eventType)
	if !ok {
		return Alert{}, fmt.Errorf("unknown event_type %q", eventType)
	}
	sample := d.sample
	return Alert{
		AlertID:   uuid.NewString(),
		OverlayID: overlayID,
		Platform:  d.platform,
		EventType: eventType,
		EventData: &EventData{
			Type:     eventType,
			Tier:     "medium",
			Value:    &sample,
			Duration: 5,
			Metadata: map[string]any{"test": true},
		},
		User:       User{ID: "all-chat-test", Name: "AllChatTester"},
		Message:    "This is a test alert",
		OccurredAt: now,
	}, nil
}
