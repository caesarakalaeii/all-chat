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

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGoalListStore struct {
	goal *alerts.GoalConfig
	list *alerts.ListConfig
}

func (f *fakeGoalListStore) GetGoalConfig(context.Context, string) (*alerts.GoalConfig, error) {
	return f.goal, nil
}

func (f *fakeGoalListStore) UpsertGoalConfig(_ context.Context, _ string, g alerts.GoalConfig) error {
	f.goal = &g
	return nil
}

func (f *fakeGoalListStore) GetListConfig(context.Context, string) (*alerts.ListConfig, error) {
	return f.list, nil
}

func (f *fakeGoalListStore) UpsertListConfig(_ context.Context, _ string, l alerts.ListConfig) error {
	f.list = &l
	return nil
}

type goalListEnv struct {
	router *gin.Engine
	store  *fakeGoalListStore
	rdb    *redis.Client
}

func newGoalListEnv(t *testing.T, overlayType string) goalListEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	overlay := func(id string) (*models.Overlay, error) {
		if id != "overlay-1" {
			return nil, errors.New("not found")
		}
		return &models.Overlay{ID: id, UserID: "user-1", OverlayType: overlayType}, nil
	}
	overlays := &mockOverlayRepository{
		getByIDFunc: func(_ context.Context, id string) (*models.Overlay, error) { return overlay(id) },
		getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
			if userID != "user-1" {
				return nil, errors.New("not found")
			}
			return overlay(id)
		},
	}
	store := &fakeGoalListStore{}
	h := NewGoalListHandler(store, overlays, rdb)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/public/:id/goal", h.HandleGetPublicGoal)
	router.GET("/:id/list/top", h.HandleGetListTop)
	authed := router.Group("/", func(c *gin.Context) { c.Set("user_id", "user-1") })
	authed.GET("/:id/goal", h.HandleGetGoal)
	authed.PUT("/:id/goal", h.HandlePutGoal)
	authed.GET("/:id/list", h.HandleGetList)
	authed.PUT("/:id/list", h.HandlePutList)
	return goalListEnv{router: router, store: store, rdb: rdb}
}

func (e goalListEnv) do(t *testing.T, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func TestPutGoalValidatesAndPublishes(t *testing.T) {
	env := newGoalListEnv(t, models.OverlayTypeGoal)
	ctx := context.Background()
	sub := env.rdb.Subscribe(ctx, "overlay:overlay-1:alerts")
	_, err := sub.Receive(ctx)
	require.NoError(t, err)

	w, _ := env.do(t, http.MethodPut, "/overlay-1/goal", map[string]any{"metric": "followers", "platform": "youtube", "target": 10})
	assert.Equal(t, http.StatusBadRequest, w.Code, "followers is Twitch-only")
	assert.Nil(t, env.store.goal)

	env.rdb.Set(ctx, alerts.GoalKey("overlay-1", "bits"), "40", 0)
	w, body := env.do(t, http.MethodPut, "/overlay-1/goal", map[string]any{"metric": "bits", "platform": "twitch", "target": 1000, "starting_value": 100, "title": "Bits"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 140, body["current"], "counter goals add counted events to starting_value")
	require.NotNil(t, env.store.goal)

	msg, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)
	var frame alerts.GoalFrame
	require.NoError(t, json.Unmarshal([]byte(msg.Payload), &frame))
	assert.Equal(t, alerts.GoalFrame{Type: "goal", OverlayID: "overlay-1", Goal: "bits", Current: 140, Target: 1000, Title: "Bits"}, frame)

	w, body = env.do(t, http.MethodPut, "/overlay-1/goal", map[string]any{"metric": "bits", "platform": "twitch", "target": 1000, "starting_value": 0, "reset_progress": true})
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 0, body["current"], "reset_progress clears counted events")
}

func TestGoalRoutesRefuseOtherKinds(t *testing.T) {
	env := newGoalListEnv(t, models.OverlayTypeList)
	w, _ := env.do(t, http.MethodPut, "/overlay-1/goal", map[string]any{"metric": "bits", "platform": "twitch", "target": 10})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w, _ = env.do(t, http.MethodGet, "/public/overlay-1/goal", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	w, _ = env.do(t, http.MethodGet, "/overlay-2/goal", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPublicGoalReportsPolledValueAndStatus(t *testing.T) {
	env := newGoalListEnv(t, models.OverlayTypeGoal)
	ctx := context.Background()
	env.store.goal = &alerts.GoalConfig{Metric: "followers", Platform: "twitch", Target: 500, StartingValue: 7}

	_, body := env.do(t, http.MethodGet, "/public/overlay-1/goal", nil)
	assert.EqualValues(t, 7, body["current"], "before the first poll, starting_value stands in")

	env.rdb.Set(ctx, alerts.GoalKey("overlay-1", "followers"), "321", 0)
	env.rdb.Set(ctx, alerts.GoalStatusKey("overlay-1"), "reconnect", 0)
	_, body = env.do(t, http.MethodGet, "/public/overlay-1/goal", nil)
	assert.EqualValues(t, 321, body["current"], "a polled count replaces starting_value")
	assert.Equal(t, "reconnect", body["status"])
	assert.Equal(t, true, body["configured"])
}

func TestListConfigAndTop(t *testing.T) {
	env := newGoalListEnv(t, models.OverlayTypeList)
	ctx := context.Background()

	_, body := env.do(t, http.MethodGet, "/overlay-1/list", nil)
	assert.Equal(t, false, body["configured"])

	w, _ := env.do(t, http.MethodPut, "/overlay-1/list", map[string]any{"category": "follows", "limit": 5})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w, _ = env.do(t, http.MethodPut, "/overlay-1/list", map[string]any{"category": "gifts", "limit": 2, "title": "Top gifters"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	env.rdb.ZAdd(ctx, alerts.LeaderboardKey("overlay-1", "gifts"),
		redis.Z{Score: 5, Member: "twitch:1"}, redis.Z{Score: 20, Member: "twitch:2"}, redis.Z{Score: 1, Member: "youtube:3"})
	env.rdb.HSet(ctx, alerts.LeaderboardNamesKey("overlay-1", "gifts"), "twitch:2", "Gifter")

	w, body = env.do(t, http.MethodGet, "/overlay-1/list/top", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "gifts", body["category"])
	assert.Equal(t, "Top gifters", body["title"])
	entries := body["entries"].([]any)
	require.Len(t, entries, 2, "the configured limit applies")
	assert.Equal(t, "Gifter", entries[0].(map[string]any)["name"])

	w, body = env.do(t, http.MethodGet, "/overlay-1/list/top?category=bits&limit=99", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "bits", body["category"])
	assert.Empty(t, body["entries"])

	w, _ = env.do(t, http.MethodGet, "/overlay-1/list/top?category=follows", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Saving a list pushes the newly chosen board with its limit and title, which
// is what tells a live renderer to switch boards.
func TestPutListPushesTheChosenBoard(t *testing.T) {
	env := newGoalListEnv(t, models.OverlayTypeList)
	ctx := context.Background()
	sub := env.rdb.Subscribe(ctx, "overlay:overlay-1:alerts")
	_, err := sub.Receive(ctx)
	require.NoError(t, err)

	w, _ := env.do(t, http.MethodPut, "/overlay-1/list", map[string]any{"category": "super_chat", "limit": 3, "title": "SC"})
	require.Equal(t, http.StatusOK, w.Code)
	msg, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"list_update","overlay_id":"overlay-1","category":"super_chat","entries":[],"limit":3,"title":"SC"}`, msg.Payload)
}
