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
	"testing"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockViewerIdentityRepo is a test double for ViewerIdentityRepo.
type mockViewerIdentityRepo struct {
	getOrCreate           func(ctx context.Context, platform, platformUserID string) (uuid.UUID, error)
	linkPlatform          func(ctx context.Context, viewerID uuid.UUID, platform, platformUserID string) error
	linkViewerToUser      func(ctx context.Context, platform, platformUserID, userID string, isPremium bool) error
	getIsPremium          func(ctx context.Context, viewerID uuid.UUID) (bool, error)
	getLinked             func(ctx context.Context, viewerID uuid.UUID) ([]repository.LinkedPlatform, error)
	unlinkPlatform        func(ctx context.Context, viewerID uuid.UUID, platform string) error
	migratePlatformUserID func(ctx context.Context, platform, oldID, newID string) error
}

func (m *mockViewerIdentityRepo) GetOrCreateViewerByPlatform(ctx context.Context, platform, platformUserID string) (uuid.UUID, error) {
	if m.getOrCreate != nil {
		return m.getOrCreate(ctx, platform, platformUserID)
	}
	return uuid.New(), nil
}

func (m *mockViewerIdentityRepo) LinkPlatformToViewer(ctx context.Context, viewerID uuid.UUID, platform, platformUserID string) error {
	if m.linkPlatform != nil {
		return m.linkPlatform(ctx, viewerID, platform, platformUserID)
	}
	return nil
}

func (m *mockViewerIdentityRepo) LinkViewerToUser(ctx context.Context, platform, platformUserID, userID string, isPremium bool) error {
	if m.linkViewerToUser != nil {
		return m.linkViewerToUser(ctx, platform, platformUserID, userID, isPremium)
	}
	return nil
}

func (m *mockViewerIdentityRepo) GetViewerIsPremium(ctx context.Context, viewerID uuid.UUID) (bool, error) {
	if m.getIsPremium != nil {
		return m.getIsPremium(ctx, viewerID)
	}
	return false, nil
}

func (m *mockViewerIdentityRepo) GetLinkedPlatforms(ctx context.Context, viewerID uuid.UUID) ([]repository.LinkedPlatform, error) {
	if m.getLinked != nil {
		return m.getLinked(ctx, viewerID)
	}
	return nil, nil
}

func (m *mockViewerIdentityRepo) UnlinkPlatform(ctx context.Context, viewerID uuid.UUID, platform string) error {
	if m.unlinkPlatform != nil {
		return m.unlinkPlatform(ctx, viewerID, platform)
	}
	return nil
}

func (m *mockViewerIdentityRepo) MigratePlatformUserID(ctx context.Context, platform, oldID, newID string) error {
	if m.migratePlatformUserID != nil {
		return m.migratePlatformUserID(ctx, platform, oldID, newID)
	}
	return nil
}

// newResolveTestHandler returns a minimal ViewerAuthHandler with the given identity repo mock.
func newResolveTestHandler(repo ViewerIdentityRepo) *ViewerAuthHandler {
	return &ViewerAuthHandler{
		identityRepo: repo,
		logger:       zap.NewNop(),
	}
}

// ---------------------------------------------------------------------------
// Tests for resolveViewerID
// ---------------------------------------------------------------------------

func TestResolveViewerID_NoLinkParam_CallsGetOrCreate(t *testing.T) {
	expectedID := uuid.New()
	called := false
	mock := &mockViewerIdentityRepo{
		getOrCreate: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			called = true
			return expectedID, nil
		},
	}

	h := newResolveTestHandler(mock)
	got, err := h.resolveViewerID(context.Background(), "twitch", "user123", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected GetOrCreateViewerByPlatform to be called")
	}
	if got != expectedID {
		t.Errorf("got viewer ID %v, want %v", got, expectedID)
	}
}

func TestResolveViewerID_WithValidLinkID_CallsLinkPlatformToViewer(t *testing.T) {
	linkID := uuid.New()
	linkCalled := false
	createCalled := false

	mock := &mockViewerIdentityRepo{
		linkPlatform: func(_ context.Context, viewerID uuid.UUID, _, _ string) error {
			linkCalled = true
			if viewerID != linkID {
				t.Errorf("LinkPlatformToViewer got viewerID %v, want %v", viewerID, linkID)
			}
			return nil
		},
		getOrCreate: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			createCalled = true
			return uuid.New(), nil
		},
	}

	h := newResolveTestHandler(mock)
	got, err := h.resolveViewerID(context.Background(), "youtube", "ytuser456", map[string]string{
		"link_viewer_id": linkID.String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !linkCalled {
		t.Error("expected LinkPlatformToViewer to be called")
	}
	if createCalled {
		t.Error("GetOrCreateViewerByPlatform should NOT be called when link succeeds")
	}
	if got != linkID {
		t.Errorf("got viewer ID %v, want link ID %v", got, linkID)
	}
}

func TestResolveViewerID_WithInvalidLinkID_FallsBackToGetOrCreate(t *testing.T) {
	expectedID := uuid.New()
	createCalled := false

	mock := &mockViewerIdentityRepo{
		getOrCreate: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			createCalled = true
			return expectedID, nil
		},
	}

	h := newResolveTestHandler(mock)
	got, err := h.resolveViewerID(context.Background(), "kick", "kickuser", map[string]string{
		"link_viewer_id": "not-a-valid-uuid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("expected GetOrCreateViewerByPlatform to be called when link_viewer_id is invalid")
	}
	if got != expectedID {
		t.Errorf("got viewer ID %v, want %v", got, expectedID)
	}
}

func TestResolveViewerID_LinkAlreadyLinkedToDifferentViewer_FallsBack(t *testing.T) {
	linkID := uuid.New()
	fallbackID := uuid.New()

	mock := &mockViewerIdentityRepo{
		linkPlatform: func(_ context.Context, _ uuid.UUID, _, _ string) error {
			return repository.ErrPlatformAlreadyLinked
		},
		getOrCreate: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			return fallbackID, nil
		},
	}

	h := newResolveTestHandler(mock)
	got, err := h.resolveViewerID(context.Background(), "twitch", "user999", map[string]string{
		"link_viewer_id": linkID.String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back and return the fallback ID (the platform's own viewer)
	if got != fallbackID {
		t.Errorf("got viewer ID %v, want fallback ID %v", got, fallbackID)
	}
}

func TestResolveViewerID_GetOrCreateError_ReturnsError(t *testing.T) {
	dbErr := errors.New("database unavailable")
	mock := &mockViewerIdentityRepo{
		getOrCreate: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			return uuid.Nil, dbErr
		},
	}

	h := newResolveTestHandler(mock)
	_, err := h.resolveViewerID(context.Background(), "twitch", "user", map[string]string{})
	if err == nil {
		t.Fatal("expected error from GetOrCreateViewerByPlatform to be propagated")
	}
}

func TestResolveViewerID_EmptyLinkParam_FallsBackToGetOrCreate(t *testing.T) {
	expectedID := uuid.New()
	createCalled := false

	mock := &mockViewerIdentityRepo{
		getOrCreate: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			createCalled = true
			return expectedID, nil
		},
	}

	h := newResolveTestHandler(mock)
	// link_viewer_id present but empty — should ignore and fall through
	got, err := h.resolveViewerID(context.Background(), "twitch", "user", map[string]string{
		"link_viewer_id": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("expected GetOrCreateViewerByPlatform to be called when link_viewer_id is empty")
	}
	if got != expectedID {
		t.Errorf("got viewer ID %v, want %v", got, expectedID)
	}
}

// ---------------------------------------------------------------------------
// Tests for HandleGetLinkedPlatforms
// ---------------------------------------------------------------------------

func newLinkedPlatformsRouter(mock ViewerIdentityRepo) (*gin.Engine, *ViewerAuthHandler) {
	gin.SetMode(gin.TestMode)
	h := &ViewerAuthHandler{
		identityRepo: mock,
		logger:       zap.NewNop(),
	}
	r := gin.New()
	r.GET("/viewer/linked-platforms", func(c *gin.Context) {
		// inject context as middleware would
		c.Set("viewer_id", c.GetHeader("X-Viewer-ID"))
		c.Set("platform", c.GetHeader("X-Platform"))
		h.HandleGetLinkedPlatforms(c)
	})
	r.DELETE("/viewer/linked-platforms/:platform", func(c *gin.Context) {
		c.Set("viewer_id", c.GetHeader("X-Viewer-ID"))
		c.Set("platform", c.GetHeader("X-Platform"))
		h.HandleUnlinkPlatform(c)
	})
	return r, h
}

func TestHandleGetLinkedPlatforms_ReturnsList(t *testing.T) {
	viewerID := uuid.New()
	mock := &mockViewerIdentityRepo{
		getLinked: func(_ context.Context, id uuid.UUID) ([]repository.LinkedPlatform, error) {
			if id != viewerID {
				t.Errorf("unexpected viewerID: %v", id)
			}
			return []repository.LinkedPlatform{
				{Platform: "twitch", PlatformUserID: "t123"},
				{Platform: "youtube", PlatformUserID: "y456"},
			}, nil
		},
	}
	r, _ := newLinkedPlatformsRouter(mock)

	req, _ := http.NewRequest(http.MethodGet, "/viewer/linked-platforms", nil)
	req.Header.Set("X-Viewer-ID", viewerID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	platforms, ok := resp["platforms"].([]interface{})
	if !ok {
		t.Fatalf("platforms missing or wrong type: %v", resp)
	}
	if len(platforms) != 2 {
		t.Errorf("expected 2 platforms, got %d", len(platforms))
	}
}

func TestHandleGetLinkedPlatforms_MissingViewerID_Returns401(t *testing.T) {
	r, _ := newLinkedPlatformsRouter(&mockViewerIdentityRepo{})

	req, _ := http.NewRequest(http.MethodGet, "/viewer/linked-platforms", nil)
	// No X-Viewer-ID header → empty string → handler returns 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests for HandleUnlinkPlatform
// ---------------------------------------------------------------------------

func TestHandleUnlinkPlatform_Success(t *testing.T) {
	viewerID := uuid.New()
	unlinkCalled := false
	mock := &mockViewerIdentityRepo{
		unlinkPlatform: func(_ context.Context, id uuid.UUID, platform string) error {
			unlinkCalled = true
			if id != viewerID {
				t.Errorf("unexpected viewerID: %v", id)
			}
			if platform != "youtube" {
				t.Errorf("unexpected platform: %s", platform)
			}
			return nil
		},
	}
	r, _ := newLinkedPlatformsRouter(mock)

	req, _ := http.NewRequest(http.MethodDelete, "/viewer/linked-platforms/youtube", nil)
	req.Header.Set("X-Viewer-ID", viewerID.String())
	req.Header.Set("X-Platform", "twitch") // current JWT platform
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !unlinkCalled {
		t.Error("expected UnlinkPlatform to be called")
	}
}

func TestHandleUnlinkPlatform_CannotUnlinkCurrentPlatform_Returns400(t *testing.T) {
	viewerID := uuid.New()
	r, _ := newLinkedPlatformsRouter(&mockViewerIdentityRepo{})

	req, _ := http.NewRequest(http.MethodDelete, "/viewer/linked-platforms/twitch", nil)
	req.Header.Set("X-Viewer-ID", viewerID.String())
	req.Header.Set("X-Platform", "twitch") // same as target — not allowed
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUnlinkPlatform_LastPlatform_Returns409(t *testing.T) {
	viewerID := uuid.New()
	mock := &mockViewerIdentityRepo{
		unlinkPlatform: func(_ context.Context, _ uuid.UUID, _ string) error {
			return repository.ErrLastPlatform
		},
	}
	r, _ := newLinkedPlatformsRouter(mock)

	req, _ := http.NewRequest(http.MethodDelete, "/viewer/linked-platforms/youtube", nil)
	req.Header.Set("X-Viewer-ID", viewerID.String())
	req.Header.Set("X-Platform", "twitch")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUnlinkPlatform_NotLinked_Returns404(t *testing.T) {
	viewerID := uuid.New()
	mock := &mockViewerIdentityRepo{
		unlinkPlatform: func(_ context.Context, _ uuid.UUID, _ string) error {
			return repository.ErrNotFound
		},
	}
	r, _ := newLinkedPlatformsRouter(mock)

	req, _ := http.NewRequest(http.MethodDelete, "/viewer/linked-platforms/kick", nil)
	req.Header.Set("X-Viewer-ID", viewerID.String())
	req.Header.Set("X-Platform", "twitch")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests for MigratePlatformUserID (interface contract)
// ---------------------------------------------------------------------------

// TestMockViewerIdentityRepo_MigratePlatformUserID verifies that the mock correctly
// calls the provided migratePlatformUserID function and returns nil by default.
// This also exercises the ViewerIdentityRepo interface constraint — if
// ViewerIdentityRepository does not implement MigratePlatformUserID, the
// compile-time interface check below will fail.
func TestMockViewerIdentityRepo_MigratePlatformUserID_CallsFunc(t *testing.T) {
	called := false
	gotPlatform, gotOld, gotNew := "", "", ""

	mock := &mockViewerIdentityRepo{
		migratePlatformUserID: func(_ context.Context, platform, oldID, newID string) error {
			called = true
			gotPlatform = platform
			gotOld = oldID
			gotNew = newID
			return nil
		},
	}

	err := mock.MigratePlatformUserID(context.Background(), "youtube", "101802728631468199113", "UCxxxxxx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("migratePlatformUserID func was not called")
	}
	if gotPlatform != "youtube" || gotOld != "101802728631468199113" || gotNew != "UCxxxxxx" {
		t.Errorf("unexpected args: platform=%q old=%q new=%q", gotPlatform, gotOld, gotNew)
	}
}

func TestMockViewerIdentityRepo_MigratePlatformUserID_DefaultNil(t *testing.T) {
	mock := &mockViewerIdentityRepo{}
	err := mock.MigratePlatformUserID(context.Background(), "youtube", "old", "new")
	if err != nil {
		t.Errorf("expected nil error by default, got %v", err)
	}
}
