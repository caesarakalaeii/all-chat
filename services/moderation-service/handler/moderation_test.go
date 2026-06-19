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

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/moderation-service/audit"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	ownerID   = "11111111-1111-1111-1111-111111111111"
	adminID   = "99999999-9999-9999-9999-999999999999"
	overlayID = "aaaaaaaa-1111-1111-1111-111111111111"
)

type fakeAuthorizer struct {
	owns     bool
	ownsErr  error
	isSource map[string]bool
	sources  []repository.Source
}

func (f *fakeAuthorizer) VerifyOverlayOwnership(context.Context, string, string) (bool, error) {
	return f.owns, f.ownsErr
}
func (f *fakeAuthorizer) IsModeratableSource(_ context.Context, _, platform, channelID string) (bool, error) {
	return f.isSource[platform+"|"+channelID], nil
}
func (f *fakeAuthorizer) ListModeratableSources(context.Context, string) ([]repository.Source, error) {
	return f.sources, nil
}

type fakeEmitter struct{ published []*mpmodels.RawChatMessage }

func (f *fakeEmitter) Publish(_ context.Context, msg *mpmodels.RawChatMessage) error {
	f.published = append(f.published, msg)
	return nil
}

type fakeRecorder struct{ entries []audit.Entry }

func (f *fakeRecorder) Record(_ context.Context, e audit.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

type fakeScopes struct{ actions []models.Action }

func (f fakeScopes) GrantedActions(context.Context, string, string, string) ([]models.Action, error) {
	return f.actions, nil
}

// fakeGate stands in for the ADR-0008 feature-gate cohort check.
type fakeGate struct {
	enabled bool
	err     error
}

func (f fakeGate) ModerationEnabled(context.Context, string) (bool, error) {
	return f.enabled, f.err
}

// fakeDispatcher records the dispatch call and returns a canned result, so handler
// tests exercise the success / reauth / no-credential branches without a platform.
type fakeDispatcher struct {
	res       models.DispatchResult
	err       error
	calls     int
	gotUserID string
	gotAction models.Action
	gotReq    models.DispatchRequest
}

func (f *fakeDispatcher) Dispatch(_ context.Context, userID string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	f.calls++
	f.gotUserID = userID
	f.gotAction = action
	f.gotReq = req
	return f.res, f.err
}

func newTestRouter(h *Handler, userID, impersonatedBy string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { // stand in for shared JWTAuth
		if userID != "" {
			c.Set("user_id", userID)
		}
		if impersonatedBy != "" {
			c.Set("impersonated_by", impersonatedBy)
		}
		c.Next()
	})
	api := r.Group("/api/v1/moderation")
	api.POST("/overlays/:id/delete", h.HandleDelete)
	api.POST("/overlays/:id/timeout", h.HandleTimeout)
	api.POST("/overlays/:id/ban", h.HandleBan)
	api.POST("/overlays/:id/unban", h.HandleUnban)
	api.GET("/overlays/:id/capabilities", h.HandleCapabilities)
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	return resp
}

func TestDelete_OwnerEmitsReflectBackAndAudits(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	h := New(auth, emitter, rec, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"twitch","channel_id":"somestreamer","native_message_id":"nm1","target_uuid":"u1"}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Len(t, emitter.published, 1, "a reflect-back deletion must be emitted")
	assert.Equal(t, "single", emitter.published[0].EventData["deletion_type"])
	assert.Equal(t, "nm1", emitter.published[0].EventData["target_msg_id"])
	assert.Equal(t, "u1", emitter.published[0].EventData["target_uuid"],
		"the internal uuid from the request is threaded through so the consumer matches without the registry")

	require.Len(t, rec.entries, 1)
	assert.Equal(t, "delete", rec.entries[0].Action)
	assert.Equal(t, audit.OutcomeDryRun, rec.entries[0].Outcome)
	assert.Equal(t, ownerID, rec.entries[0].ActorUserID)
	assert.Empty(t, rec.entries[0].ImpersonatedBy)
}

func TestDelete_NotOwnerIsDeniedAndAudited(t *testing.T) {
	auth := &fakeAuthorizer{owns: false}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	h := New(auth, emitter, rec, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, strangerID(), "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"twitch","channel_id":"somestreamer","native_message_id":"nm1"}`)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Empty(t, emitter.published, "a denied request must not emit a deletion")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeDenied, rec.entries[0].Outcome)
}

func TestDelete_TikTokIsUnsupported(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"tiktok|tt": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	h := New(auth, emitter, rec, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"tiktok","channel_id":"tt","native_message_id":"nm1"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Empty(t, emitter.published)
}

func TestDelete_ChannelNotASource(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{}} // no sources match
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"twitch","channel_id":"not-on-overlay","native_message_id":"nm1"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
}

func TestTimeout_UnderImpersonationAttributesAdmin(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	h := New(auth, emitter, rec, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, ownerID, adminID) // admin impersonating the owner

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/timeout",
		`{"platform":"twitch","channel_id":"somestreamer","target_user_id":"42","target_username":"bad","duration_seconds":600,"reason":"spam"}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Len(t, emitter.published, 1)
	assert.Equal(t, "batch", emitter.published[0].EventData["deletion_type"])
	assert.Equal(t, 600, emitter.published[0].EventData["ban_duration"], "timeout must carry ban_duration")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, "timeout", rec.entries[0].Action)
	assert.Equal(t, ownerID, rec.entries[0].ActorUserID, "action runs as the impersonated owner")
	assert.Equal(t, adminID, rec.entries[0].ImpersonatedBy, "the real admin must be recorded")
}

func TestCapabilities_OwnerSeesPerSourceState(t *testing.T) {
	auth := &fakeAuthorizer{
		owns: true,
		sources: []repository.Source{
			{Platform: "twitch", ChannelID: "somestreamer", ChannelName: "SomeStreamer"},
			{Platform: "tiktok", ChannelID: "tt", ChannelName: "TikTokUser"},
		},
	}
	// Owner has granted delete+ban on twitch (fake returns these for every source;
	// tiktok is filtered out earlier by PlatformSupported).
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, fakeScopes{actions: []models.Action{models.ActionDelete, models.ActionBan}}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodGet, "/api/v1/moderation/overlays/"+overlayID+"/capabilities", "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.True(t, caps.IsOwner)
	assert.True(t, caps.Enabled, "default gate (OpenGate) enables moderation for owners")
	require.Len(t, caps.Sources, 2)

	bySource := map[string]models.SourceCapability{}
	for _, s := range caps.Sources {
		bySource[s.Platform] = s
	}
	assert.True(t, bySource["twitch"].Moderatable)
	assert.ElementsMatch(t, []models.Action{models.ActionDelete, models.ActionBan}, bySource["twitch"].Actions)
	assert.False(t, bySource["tiktok"].Moderatable)
	assert.Equal(t, models.ReasonUnsupportedPlatform, bySource["tiktok"].Reason)
}

func TestCapabilities_MissingScopeWhenNoGrant(t *testing.T) {
	auth := &fakeAuthorizer{
		owns:    true,
		sources: []repository.Source{{Platform: "twitch", ChannelID: "somestreamer", ChannelName: "SomeStreamer"}},
	}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodGet, "/api/v1/moderation/overlays/"+overlayID+"/capabilities", "")
	require.Equal(t, http.StatusOK, resp.Code)

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].Moderatable)
	assert.Equal(t, models.ReasonMissingScope, caps.Sources[0].Reason)
}

func TestCapabilities_NonOwnerGetsEmptyReadOnly(t *testing.T) {
	auth := &fakeAuthorizer{owns: false, sources: []repository.Source{{Platform: "twitch", ChannelID: "x", ChannelName: "X"}}}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	r := newTestRouter(h, strangerID(), "")

	resp := do(r, http.MethodGet, "/api/v1/moderation/overlays/"+overlayID+"/capabilities", "")
	require.Equal(t, http.StatusOK, resp.Code)

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.False(t, caps.IsOwner)
	assert.Empty(t, caps.Sources, "non-owners must not see the source list")
}

func TestCapabilities_FeatureGateDisabledReportsNotEnabled(t *testing.T) {
	auth := &fakeAuthorizer{
		owns:    true,
		sources: []repository.Source{{Platform: "twitch", ChannelID: "somestreamer", ChannelName: "SomeStreamer"}},
	}
	// Owner holds the scopes, but the moderation feature gate is closed for this
	// user (not in the rollout cohort) — capabilities must report enabled:false so
	// the dashboard hides the controls instead of offering actions that 403.
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, fakeScopes{actions: []models.Action{models.ActionDelete}}, DryRunDispatcher{}, zap.NewNop())
	h.SetFeatureGate(fakeGate{enabled: false})
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodGet, "/api/v1/moderation/overlays/"+overlayID+"/capabilities", "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.True(t, caps.IsOwner)
	assert.False(t, caps.Enabled, "feature gate closed → moderation not enabled for this user")
}

func TestCapabilities_FeatureGateErrorFailsClosed(t *testing.T) {
	auth := &fakeAuthorizer{
		owns:    true,
		sources: []repository.Source{{Platform: "twitch", ChannelID: "somestreamer", ChannelName: "SomeStreamer"}},
	}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, fakeScopes{actions: []models.Action{models.ActionDelete}}, DryRunDispatcher{}, zap.NewNop())
	h.SetFeatureGate(fakeGate{err: assertAnErr{}})
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodGet, "/api/v1/moderation/overlays/"+overlayID+"/capabilities", "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.False(t, caps.Enabled, "a gate lookup error must fail closed (moderation disabled)")
}

func strangerID() string { return "22222222-2222-2222-2222-222222222222" }

// --- Real-platform dispatch paths (Phase 1) ---------------------------------

func TestDelete_PerformedEmitsReflectBackAndAuditsSuccess(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	disp := &fakeDispatcher{res: models.DispatchResult{Outcome: models.DispatchPerformed}}
	h := New(auth, emitter, rec, NoScopeChecker{}, disp, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"twitch","channel_id":"somestreamer","native_message_id":"nm1","target_uuid":"u1"}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"dry_run":false`)
	// The native message id must reach the platform dispatcher unchanged.
	require.Equal(t, 1, disp.calls)
	assert.Equal(t, models.ActionDelete, disp.gotAction)
	assert.Equal(t, "nm1", disp.gotReq.NativeMessageID)
	assert.Equal(t, ownerID, disp.gotUserID)
	require.Len(t, emitter.published, 1, "a performed action still emits the reflect-back")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeSuccess, rec.entries[0].Outcome)
}

func TestDelete_ReauthRequiredReturns403AndDoesNotEmit(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	disp := &fakeDispatcher{res: models.DispatchResult{
		Outcome:       models.DispatchReauthRequired,
		MissingScopes: []string{models.ScopeTwitchManageMessages},
	}}
	h := New(auth, emitter, rec, NoScopeChecker{}, disp, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"twitch","channel_id":"somestreamer","native_message_id":"nm1"}`)

	require.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"requires_reauth":true`)
	assert.Contains(t, resp.Body.String(), models.ScopeTwitchManageMessages)
	assert.Empty(t, emitter.published, "a re-consent-required action must not emit a deletion")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeReauthRequired, rec.entries[0].Outcome)
}

func TestDelete_NoCredentialReturns422(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	disp := &fakeDispatcher{res: models.DispatchResult{Outcome: models.DispatchNoCredential}}
	h := New(auth, emitter, rec, NoScopeChecker{}, disp, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/delete",
		`{"platform":"twitch","channel_id":"somestreamer","native_message_id":"nm1"}`)

	require.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
	assert.Empty(t, emitter.published)
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeNoCredential, rec.entries[0].Outcome)
}

func TestTimeout_PerformedThreadsDurationToDispatch(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	disp := &fakeDispatcher{res: models.DispatchResult{Outcome: models.DispatchPerformed}}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, disp, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/timeout",
		`{"platform":"twitch","channel_id":"somestreamer","target_user_id":"42","duration_seconds":600,"reason":"spam"}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Equal(t, models.ActionTimeout, disp.gotAction)
	assert.Equal(t, 600, disp.gotReq.DurationSeconds)
	assert.Equal(t, "42", disp.gotReq.TargetUserID)
}

func TestUnban_PerformedAuditsSuccessWithoutReflectBack(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	disp := &fakeDispatcher{res: models.DispatchResult{Outcome: models.DispatchPerformed}}
	h := New(auth, emitter, rec, NoScopeChecker{}, disp, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/unban",
		`{"platform":"twitch","channel_id":"somestreamer","target_user_id":"42"}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Equal(t, models.ActionUnban, disp.gotAction)
	assert.Empty(t, emitter.published, "unban deletes nothing, so no reflect-back is emitted")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeSuccess, rec.entries[0].Outcome)
}

func TestDispatch_UnexpectedErrorReturns502AndAuditsPlatformError(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	disp := &fakeDispatcher{err: assertAnErr{}}
	h := New(auth, emitter, rec, NoScopeChecker{}, disp, zap.NewNop())
	r := newTestRouter(h, ownerID, "")

	resp := do(r, http.MethodPost, "/api/v1/moderation/overlays/"+overlayID+"/ban",
		`{"platform":"twitch","channel_id":"somestreamer","target_user_id":"42"}`)

	require.Equal(t, http.StatusBadGateway, resp.Code, resp.Body.String())
	assert.Empty(t, emitter.published)
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomePlatformError, rec.entries[0].Outcome)
}

type assertAnErr struct{}

func (assertAnErr) Error() string { return "boom" }
