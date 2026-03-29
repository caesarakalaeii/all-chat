package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockViewerIdentityRepo is a test double for ViewerIdentityRepo.
type mockViewerIdentityRepo struct {
	getOrCreate       func(ctx context.Context, platform, platformUserID string) (uuid.UUID, error)
	linkPlatform      func(ctx context.Context, viewerID uuid.UUID, platform, platformUserID string) error
	linkViewerToUser  func(ctx context.Context, platform, platformUserID, userID string, isPremium bool) error
	getIsPremium      func(ctx context.Context, viewerID uuid.UUID) (bool, error)
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
