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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// platformRows satisfies pgx.Rows over a fixed slice of PlatformInfo.
type platformRows struct {
	entries []PlatformInfo
	index   int
}

func (r *platformRows) Close()                                       {}
func (r *platformRows) Err() error                                   { return nil }
func (r *platformRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *platformRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *platformRows) Next() bool                                   { return r.index < len(r.entries) }
func (r *platformRows) Values() ([]interface{}, error)               { return nil, nil }
func (r *platformRows) RawValues() [][]byte                          { return nil }
func (r *platformRows) Conn() *pgx.Conn                              { return nil }

func (r *platformRows) Scan(dest ...interface{}) error {
	if r.index >= len(r.entries) {
		return pgx.ErrNoRows
	}
	e := r.entries[r.index]
	r.index++
	if len(dest) >= 4 {
		*dest[0].(*string) = e.Platform
		*dest[1].(*string) = e.ChannelID
		*dest[2].(*string) = e.ChannelName
		*dest[3].(*bool) = e.IsActive
	}
	return nil
}

// boolRow satisfies pgx.Row for the EXISTS probe behind viewer_public.
type boolRow struct {
	value bool
	err   error
}

func (r boolRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) >= 1 {
		*dest[0].(*bool) = r.value
	}
	return nil
}

// viewerPublicDB routes the platform-source Query to a fixed row set and the
// EXISTS QueryRow to a configurable result, so the two branches that feed the
// response can be driven independently.
type viewerPublicDB struct {
	platforms     []PlatformInfo
	viewerPublic  bool
	viewerPubErr  error
	queryRowSQL   []string
	queryRowCount int
}

func (d *viewerPublicDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return &platformRows{entries: d.platforms}, nil
}

func (d *viewerPublicDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	d.queryRowCount++
	d.queryRowSQL = append(d.queryRowSQL, sql)
	return boolRow{value: d.viewerPublic, err: d.viewerPubErr}
}

func newViewerPublicRouter(t *testing.T, db *viewerPublicDB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	userRepo := new(MockUserRepository)
	userRepo.On("GetByUsername", mock.Anything, mock.Anything).
		Return(&models.User{ID: "user-1", Username: "streamer"}, nil)

	h := NewStreamerInfoHandler(zap.NewNop(), userRepo, db)
	router := gin.New()
	router.GET("/streamer/:username", h.HandleGetStreamerInfo)
	return router
}

func getStreamerInfo(t *testing.T, router *gin.Engine) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/streamer/streamer", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body map[string]any
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body %q: %v", w.Body.String(), err)
		}
	}
	return w, body
}

// A streamer whose overlay the gateway would accept a viewer connection for
// reports viewer_public: true.
func TestHandleGetStreamerInfo_ViewerPublicTrue(t *testing.T) {
	db := &viewerPublicDB{
		platforms:    []PlatformInfo{{Platform: "twitch", ChannelID: "c1", ChannelName: "Chan", IsActive: true}},
		viewerPublic: true,
	}
	w, body := getStreamerInfo(t, newViewerPublicRouter(t, db))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if body["viewer_public"] != true {
		t.Errorf("expected viewer_public true, got %#v", body["viewer_public"])
	}
	// The overlay ID must never leave the database on this path.
	if _, present := body["overlay_id"]; present {
		t.Error("overlay_id must not appear in the streamer-info response")
	}
}

// viewer_public must always be on the wire, including when false, so a client
// can distinguish "explicitly not public" from "field absent / old gateway".
func TestHandleGetStreamerInfo_ViewerPublicFalseIsSerialised(t *testing.T) {
	db := &viewerPublicDB{
		platforms:    []PlatformInfo{{Platform: "kick", ChannelID: "c2", ChannelName: "Chan2", IsActive: false}},
		viewerPublic: false,
	}
	w, body := getStreamerInfo(t, newViewerPublicRouter(t, db))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	val, present := body["viewer_public"]
	if !present {
		t.Fatal("viewer_public must be present even when false (no omitempty)")
	}
	if val != false {
		t.Errorf("expected viewer_public false, got %#v", val)
	}
}

// A failure resolving the flag degrades to false rather than failing the whole
// request; the platform list is the primary payload. Clients are required to
// treat a false from anything other than a healthy 200 as a transport problem.
func TestHandleGetStreamerInfo_ViewerPublicProbeErrorDegradesToFalse(t *testing.T) {
	db := &viewerPublicDB{
		platforms:    []PlatformInfo{{Platform: "twitch", ChannelID: "c1", ChannelName: "Chan", IsActive: true}},
		viewerPubErr: errors.New("connection refused"),
	}
	w, body := getStreamerInfo(t, newViewerPublicRouter(t, db))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite flag probe failure, got %d", w.Code)
	}
	if body["viewer_public"] != false {
		t.Errorf("expected viewer_public false on probe error, got %#v", body["viewer_public"])
	}
	if len(body["platforms"].([]any)) != 1 {
		t.Error("platform list must still be returned when the flag probe fails")
	}
}

// The flag is derived from the same predicate the api-gateway's
// GetPublicOverlayByUsername uses. If these drift, a client can be told it may
// connect and then be rejected at the upgrade (or vice versa), which is exactly
// the ambiguity this flag exists to remove.
func TestViewerPublicQueryMatchesGatewayPredicate(t *testing.T) {
	for _, clause := range []string{
		"o.is_active = true",
		"o.is_public_for_viewers = true",
		"u.is_banned = false",
	} {
		if !strings.Contains(viewerPublicQuery, clause) {
			t.Errorf("viewer_public query must mirror the gateway predicate; missing %q", clause)
		}
	}
	if strings.Contains(viewerPublicQuery, "SELECT o.id") {
		t.Error("viewer_public probe must select existence only, never the overlay ID")
	}
}
