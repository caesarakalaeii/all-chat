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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type memCmdStore struct {
	cmds []models.ChatCommand
}

func (m *memCmdStore) ListChatCommands(_ context.Context, userID uuid.UUID) ([]models.ChatCommand, error) {
	out := []models.ChatCommand{}
	for _, c := range m.cmds {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memCmdStore) CountChatCommands(ctx context.Context, userID uuid.UUID) (int, error) {
	l, _ := m.ListChatCommands(ctx, userID)
	return len(l), nil
}

func (m *memCmdStore) CreateChatCommand(_ context.Context, c models.ChatCommand) (models.ChatCommand, error) {
	for _, e := range m.cmds {
		if e.UserID == c.UserID && e.Kind == c.Kind && strings.EqualFold(e.Trigger, c.Trigger) {
			return models.ChatCommand{}, repository.ErrDuplicateTrigger
		}
	}
	c.ID = uuid.New()
	m.cmds = append(m.cmds, c)
	return c, nil
}

func (m *memCmdStore) UpdateChatCommand(_ context.Context, c models.ChatCommand) (models.ChatCommand, error) {
	for i, e := range m.cmds {
		if e.ID == c.ID && e.UserID == c.UserID {
			m.cmds[i] = c
			return c, nil
		}
	}
	return models.ChatCommand{}, repository.ErrNotFound
}

func (m *memCmdStore) DeleteChatCommand(_ context.Context, userID, id uuid.UUID) error {
	for i, e := range m.cmds {
		if e.ID == id && e.UserID == userID {
			m.cmds = append(m.cmds[:i], m.cmds[i+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *memCmdStore) OwnerStatus(context.Context, uuid.UUID) (repository.ChatCommandOwnerStatus, error) {
	return repository.ChatCommandOwnerStatus{TwitchConnected: true, TwitchCanSend: true}, nil
}

var testOwner = uuid.MustParse("22222222-2222-2222-2222-222222222222")

func newCmdRouter(store *memCmdStore, changes *int, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewChatCommands(store, func() { *changes++ }, zap.NewNop())
	r := gin.New()
	g := r.Group("/", func(c *gin.Context) {
		if userID != "" {
			c.Set("user_id", userID)
		}
	})
	g.GET("/chat-commands", h.List)
	g.POST("/chat-commands", h.Create)
	g.PUT("/chat-commands/:cid", h.Update)
	g.DELETE("/chat-commands/:cid", h.Delete)
	g.POST("/chat-commands/preview", h.Preview)
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestChatCommandsCRUD(t *testing.T) {
	store := &memCmdStore{}
	changes := 0
	r := newCmdRouter(store, &changes, testOwner.String())

	w := do(r, http.MethodPost, "/chat-commands", `{"kind":"command","trigger":" !Hello ","response":"hi {user}","min_role":"everyone","cooldown_seconds":5,"enabled":true}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.ChatCommand
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "!Hello", created.Trigger, "trigger is trimmed")
	assert.Equal(t, 1, changes, "a write resyncs the forwarded channel set")

	w = do(r, http.MethodPost, "/chat-commands", `{"kind":"command","trigger":"!hello","response":"again","min_role":"everyone"}`)
	assert.Equal(t, http.StatusConflict, w.Code)

	w = do(r, http.MethodPut, "/chat-commands/"+created.ID.String(), `{"kind":"command","trigger":"!hello","response":"changed","min_role":"vip","cooldown_seconds":0,"enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "changed", store.cmds[0].Response)
	assert.Equal(t, "vip", store.cmds[0].MinRole)

	w = do(r, http.MethodGet, "/chat-commands", "")
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Commands []models.ChatCommand              `json:"commands"`
		Status   repository.ChatCommandOwnerStatus `json:"status"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list.Commands, 1)
	assert.True(t, list.Status.TwitchCanSend)

	// Another streamer cannot touch it.
	other := newCmdRouter(store, &changes, uuid.NewString())
	assert.Equal(t, http.StatusNotFound, do(other, http.MethodDelete, "/chat-commands/"+created.ID.String(), "").Code)

	assert.Equal(t, http.StatusNoContent, do(r, http.MethodDelete, "/chat-commands/"+created.ID.String(), "").Code)
	assert.Empty(t, store.cmds)
}

func TestChatCommandsValidation(t *testing.T) {
	r := newCmdRouter(&memCmdStore{}, new(int), testOwner.String())
	bad := []string{
		`{"kind":"macro","trigger":"!x","response":"y","min_role":"everyone"}`,
		`{"kind":"command","trigger":"two words","response":"y","min_role":"everyone"}`,
		`{"kind":"command","trigger":"   ","response":"y","min_role":"everyone"}`,
		`{"kind":"keyword","trigger":"a","response":"y","min_role":"everyone"}`,
		`{"kind":"command","trigger":"!x","response":"","create_clip":false,"min_role":"everyone"}`,
		`{"kind":"command","trigger":"!x","response":"y","min_role":"admin"}`,
		`{"kind":"command","trigger":"!x","response":"y","min_role":"everyone","cooldown_seconds":-1}`,
		`{"kind":"command","trigger":"!x","response":"` + strings.Repeat("y", 2001) + `","min_role":"everyone"}`,
		`{"kind":"command","trigger":"!x","image_url":"` + strings.Repeat("y", 1001) + `","min_role":"everyone"}`,
		`{"kind":"command","trigger":"!x","image_url":"   ","min_role":"everyone"}`,
	}
	for _, body := range bad {
		assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPost, "/chat-commands", body).Code, body[:min(len(body), 80)])
	}
	ok := `{"kind":"keyword","trigger":"clip that","create_clip":true,"min_role":"everyone"}`
	assert.Equal(t, http.StatusCreated, do(r, http.MethodPost, "/chat-commands", ok).Code)
	imageOnly := `{"kind":"command","trigger":"!hype","image_url":"https://img.example.com/hype.gif","min_role":"everyone"}`
	assert.Equal(t, http.StatusCreated, do(r, http.MethodPost, "/chat-commands", imageOnly).Code)
}

func TestChatCommandsCap(t *testing.T) {
	store := &memCmdStore{}
	for i := 0; i < maxChatCommandsPerUser; i++ {
		store.cmds = append(store.cmds, models.ChatCommand{ID: uuid.New(), UserID: testOwner})
	}
	r := newCmdRouter(store, new(int), testOwner.String())
	w := do(r, http.MethodPost, "/chat-commands", `{"kind":"command","trigger":"!one-more","response":"x","min_role":"everyone"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestChatCommandsRequireStreamer(t *testing.T) {
	r := newCmdRouter(&memCmdStore{}, new(int), "")
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodGet, "/chat-commands", "").Code)
}

func TestChatCommandsPreview(t *testing.T) {
	r := newCmdRouter(&memCmdStore{}, new(int), testOwner.String())
	w := do(r, http.MethodPost, "/chat-commands/preview", `{"response":"hi {touser} {js args.length * 2} {js nope()}","input":"@Friend extra","create_clip":true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Output string   `json:"output"`
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "hi Friend 4 [js error]", out.Output)
	require.Len(t, out.Errors, 1)
	assert.Contains(t, out.Errors[0], "nope")
}

// The preview renders the image the way chat would: with the chatter's input
// withheld, and empty (with the reason) when it is not an https URL.
func TestChatCommandsPreviewImage(t *testing.T) {
	r := newCmdRouter(&memCmdStore{}, new(int), testOwner.String())
	for body, want := range map[string]struct {
		image string
		errs  int
	}{
		`{"response":"","image":"https://img.example.com/{1}.gif","input":"evil"}`: {"https://img.example.com/.gif", 0},
		`{"response":"hi","image":"http://img.example.com/x.gif"}`:                 {"", 1},
		`{"response":"hi"}`: {"", 0},
	} {
		w := do(r, http.MethodPost, "/chat-commands/preview", body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out struct {
			Image  string   `json:"image"`
			Errors []string `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		assert.Equal(t, want.image, out.Image, body)
		assert.Len(t, out.Errors, want.errs, body)
	}
}
