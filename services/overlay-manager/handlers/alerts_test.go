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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAlertProfileStore struct {
	rows     []alerts.Profile
	upserted []alerts.Profile
	calls    int
}

func (f *fakeAlertProfileStore) ListAlertProfiles(_ context.Context, _ string) ([]alerts.Profile, error) {
	return f.rows, nil
}

func (f *fakeAlertProfileStore) UpsertAlertProfiles(_ context.Context, _ string, p []alerts.Profile) error {
	f.calls++
	f.upserted = p
	return nil
}

type alertsTestEnv struct {
	router *gin.Engine
	store  *fakeAlertProfileStore
	redis  *redis.Client
}

// newAlertsTestEnv serves the alert routes for user-1, who owns overlay-1 of
// the given kind; every other overlay id is somebody else's.
func newAlertsTestEnv(t *testing.T, overlayType string) alertsTestEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := &fakeAlertProfileStore{}
	overlays := &mockOverlayRepository{
		getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
			if id != "overlay-1" || userID != "user-1" {
				return nil, errors.New("not found")
			}
			return &models.Overlay{ID: id, UserID: userID, OverlayType: overlayType}, nil
		},
	}
	h := NewAlertsHandler(store, overlays, rdb)
	h.now = func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "user-1") })
	router.GET("/:id/alerts/profiles", h.HandleGetProfiles)
	router.PUT("/:id/alerts/profiles", h.HandlePutProfiles)
	router.POST("/:id/alerts/test", h.HandleTestAlert)
	return alertsTestEnv{router: router, store: store, redis: rdb}
}

func (e alertsTestEnv) do(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func decodeProfiles(t *testing.T, w *httptest.ResponseRecorder) []alerts.Profile {
	t.Helper()
	var body struct {
		Profiles []alerts.Profile `json:"profiles"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Profiles
}

func TestGetAlertProfiles_NoRowsReturnsEveryDefault(t *testing.T) {
	env := newAlertsTestEnv(t, models.OverlayTypeAlerts)

	w := env.do(http.MethodGet, "/overlay-1/alerts/profiles", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := decodeProfiles(t, w)
	require.Len(t, got, len(alerts.EventTypes()))
	for i, et := range alerts.EventTypes() {
		want, _ := alerts.DefaultProfile(et)
		assert.Equal(t, want, got[i])
	}
}

func TestGetAlertProfiles_StoredRowsOverrideDefaults(t *testing.T) {
	env := newAlertsTestEnv(t, models.OverlayTypeAlerts)
	custom, _ := alerts.DefaultProfile("bits")
	custom.TextTemplate = "{username} threw {amount}"
	custom.SoundURL = "https://media.allch.at/u/ding.mp3"
	env.store.rows = []alerts.Profile{custom}

	got := decodeProfiles(t, env.do(http.MethodGet, "/overlay-1/alerts/profiles", nil))

	require.Len(t, got, len(alerts.EventTypes()))
	var bits alerts.Profile
	for _, p := range got {
		if p.EventType == "bits" {
			bits = p
		}
	}
	assert.Equal(t, custom, bits)
}

func TestAlertRoutes_RejectNonOwner(t *testing.T) {
	env := newAlertsTestEnv(t, models.OverlayTypeAlerts)
	p, _ := alerts.DefaultProfile("bits")

	assert.Equal(t, http.StatusNotFound, env.do(http.MethodGet, "/overlay-2/alerts/profiles", nil).Code)
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPut, "/overlay-2/alerts/profiles", gin.H{"profiles": []alerts.Profile{p}}).Code)
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, "/overlay-2/alerts/test", gin.H{"event_type": "bits"}).Code)
	assert.Zero(t, env.store.calls)
}

func TestPutAlertProfiles_UpsertsValidProfiles(t *testing.T) {
	env := newAlertsTestEnv(t, models.OverlayTypeAlerts)
	p, _ := alerts.DefaultProfile("raid")
	p.Volume = 40
	p.Enabled = false

	w := env.do(http.MethodPut, "/overlay-1/alerts/profiles", gin.H{"profiles": []alerts.Profile{p}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []alerts.Profile{p}, env.store.upserted)
}

func TestPutAlertProfiles_RejectsInvalidInput(t *testing.T) {
	valid, _ := alerts.DefaultProfile("bits")
	unknown := valid
	unknown.EventType = "webcam"
	loud := valid
	loud.Volume = 500

	cases := map[string]any{
		"unknown event_type": gin.H{"profiles": []alerts.Profile{unknown}},
		"invalid volume":     gin.H{"profiles": []alerts.Profile{loud}},
		"duplicate type":     gin.H{"profiles": []alerts.Profile{valid, valid}},
		"no profiles field":  gin.H{},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			env := newAlertsTestEnv(t, models.OverlayTypeAlerts)
			w := env.do(http.MethodPut, "/overlay-1/alerts/profiles", body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Zero(t, env.store.calls, "nothing may be written when any profile is invalid")
		})
	}
}

func TestPutAlertProfiles_OnlyOnAlertsOverlays(t *testing.T) {
	p, _ := alerts.DefaultProfile("bits")
	for _, kind := range []string{models.OverlayTypeChat, models.OverlayTypeGoal, models.OverlayTypeList} {
		t.Run(kind, func(t *testing.T) {
			env := newAlertsTestEnv(t, kind)
			w := env.do(http.MethodPut, "/overlay-1/alerts/profiles", gin.H{"profiles": []alerts.Profile{p}})
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Zero(t, env.store.calls)
		})
	}
}

func TestTestAlert_PublishesTheAlertEnvelope(t *testing.T) {
	env := newAlertsTestEnv(t, models.OverlayTypeAlerts)
	sub := env.redis.Subscribe(context.Background(), "overlay:overlay-1:alerts")
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	require.NoError(t, err)

	w := env.do(http.MethodPost, "/overlay-1/alerts/test", gin.H{"event_type": "bits"})
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	select {
	case msg := <-sub.Channel():
		var a alerts.Alert
		require.NoError(t, json.Unmarshal([]byte(msg.Payload), &a))
		assert.Equal(t, "overlay-1", a.OverlayID)
		assert.Equal(t, "bits", a.EventType)
		assert.Equal(t, "twitch", a.Platform)
		assert.NotEmpty(t, a.AlertID)
		assert.Equal(t, true, a.EventData.Metadata["test"])

		var resp map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, a.AlertID, resp["alert_id"])
	case <-time.After(2 * time.Second):
		t.Fatal("no alert published on overlay:overlay-1:alerts")
	}
}

func TestTestAlert_RejectsUnknownEventTypeAndChatOverlays(t *testing.T) {
	env := newAlertsTestEnv(t, models.OverlayTypeAlerts)
	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPost, "/overlay-1/alerts/test", gin.H{"event_type": "webcam"}).Code)
	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPost, "/overlay-1/alerts/test", gin.H{}).Code)

	chat := newAlertsTestEnv(t, models.OverlayTypeChat)
	assert.Equal(t, http.StatusBadRequest, chat.do(http.MethodPost, "/overlay-1/alerts/test", gin.H{"event_type": "bits"}).Code,
		"a chat overlay has no alerts channel subscribed, so a test alert would vanish")
}
