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
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	ttspkg "github.com/caesar/all-chat/services/overlay-manager/tts"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ---- Constants -------------------------------------------------------------

const (
	// ttsRateLimitPerMinute caps per-overlay POST /tts calls. 60/min/overlay
	// is a billing-abuse safety net; if legitimate traffic hits this ceiling
	// raise it here — the value is intentionally centralised for ops tuning.
	ttsRateLimitPerMinute = 60

	// defaultElevenLabsBaseURL is the production upstream. Tests override
	// TTSHandler.elevenLabsBaseURL to point at a local httptest.Server.
	defaultElevenLabsBaseURL = "https://api.elevenlabs.io"

	// maxTTSText is a server-side safety ceiling on POST /tts text length.
	// Clients are expected to enforce tts_max_message_chars (default 200);
	// this is a defence in depth for clients that skip the check.
	maxTTSText = 5000

	// ttsModel is the ElevenLabs model passed to the text-to-speech endpoint.
	ttsModel = "eleven_multilingual_v2"

	// ttsSampleText is the fixed phrase used by HandleTestKey (D-21).
	ttsSampleText = "Hello, this is how your chat will sound."

	// ttsHTTPTimeout caps outbound ElevenLabs calls (voices, subscription,
	// single-shot test sample). POST /tts streaming MAY exceed this because
	// we rely on context cancellation instead.
	ttsHTTPTimeout = 30 * time.Second
)

// ---- Narrow interfaces (test seams) ----------------------------------------

// ttsConfigStore is the narrow surface of repository.TTSConfigRepository
// that the handler needs. Kept minimal so the test suite can mock without
// touching pgx.
type ttsConfigStore interface {
	GetByOverlayID(ctx context.Context, overlayID string) (*models.TTSConfig, error)
	CreateOrUpdate(ctx context.Context, overlayID string, encryptedKey []byte, voiceID string) (*models.TTSConfig, error)
	UpdateVoiceID(ctx context.Context, overlayID string, voiceID string) error
	Delete(ctx context.Context, overlayID string) error
	RotateSigningSecret(ctx context.Context, overlayID string) ([]byte, error)
}

// overlayOwnershipChecker is the narrow surface the handler needs from the
// overlay repo to enforce per-overlay ownership.
type overlayOwnershipChecker interface {
	GetByIDAndUserID(ctx context.Context, overlayID, userID string) (*models.Overlay, error)
}

// aesCipher is the narrow surface the handler needs from
// shared/encryption.MultiKeyEncryptor (or AESEncryptor in tests).
type aesCipher interface {
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

// ---- Handler ---------------------------------------------------------------

// TTSHandler implements the seven Phase 13 TTS endpoints:
//
//   POST   /:id/tts-config               (D-11) premium
//   DELETE /:id/tts-config               (D-12) premium
//   POST   /:id/tts-config/rotate-token  (D-13) premium
//   GET    /:id/tts-voices               (D-14) premium
//   POST   /:id/tts-config/test          (D-15) premium
//   POST   /:id/tts                      (D-16) tts_token JWT
//   GET    /:id/tts-config               (Research Open Question 3) authed
//
// All endpoints except POST /:id/tts go through the user JWT + RequirePremium
// gate; POST /:id/tts verifies a per-overlay tts_token JWT instead because
// OBS browser sources don't carry the user session.
type TTSHandler struct {
	repo          ttsConfigStore
	overlays      overlayOwnershipChecker
	cipher        aesCipher
	httpClient    *http.Client
	logger        *zap.Logger
	publicBaseURL string
	// elevenLabsBaseURL is exported only for tests via the whitebox package
	// access. Production code uses defaultElevenLabsBaseURL.
	elevenLabsBaseURL string

	rateMu      sync.Mutex
	rateBuckets map[string]*rateBucket
}

type rateBucket struct {
	count       int
	windowStart time.Time
}

// NewTTSHandler constructs a handler. publicBaseURL is the external URL the
// browser reaches (e.g. "https://allch.at"); it is used to build the
// obs_url returned by HandleGetTTSConfig / HandleRotateToken.
func NewTTSHandler(repo ttsConfigStore, overlays overlayOwnershipChecker, cipher aesCipher, publicBaseURL string, logger *zap.Logger) *TTSHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TTSHandler{
		repo:              repo,
		overlays:          overlays,
		cipher:            cipher,
		httpClient:        &http.Client{Timeout: ttsHTTPTimeout},
		logger:            logger,
		publicBaseURL:     publicBaseURL,
		elevenLabsBaseURL: defaultElevenLabsBaseURL,
		rateBuckets:       make(map[string]*rateBucket),
	}
}

// ---- Helpers ---------------------------------------------------------------

// checkOwnership returns (userID, overlayID, ok). On failure it writes the
// appropriate 401/400/404 response and returns ok=false.
func (h *TTSHandler) checkOwnership(c *gin.Context) (string, string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return "", "", false
	}
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return "", "", false
	}
	uid, _ := userID.(string)
	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, uid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return "", "", false
	}
	return uid, overlayID, true
}

// checkRateLimit returns true if the request is allowed. Window is a fixed
// 60-second epoch per overlay. When the window elapses the bucket resets.
// Reset-on-restart is acceptable (T-13-04).
func (h *TTSHandler) checkRateLimit(overlayID string) bool {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	now := time.Now()
	b, ok := h.rateBuckets[overlayID]
	if !ok || now.Sub(b.windowStart) >= time.Minute {
		h.rateBuckets[overlayID] = &rateBucket{count: 1, windowStart: now}
		return true
	}
	if b.count >= ttsRateLimitPerMinute {
		return false
	}
	b.count++
	return true
}

// buildOBSURL produces the deep link with the tts_token embedded.
func (h *TTSHandler) buildOBSURL(overlayID string, jwtToken string) string {
	return fmt.Sprintf("%s/overlay/%s?tts_token=%s", strings.TrimRight(h.publicBaseURL, "/"), overlayID, jwtToken)
}

// ---- ElevenLabs error mapping ----------------------------------------------

// elevenLabsErrorBody mirrors ElevenLabs' /detail error envelope. The shape is:
//
//	{"detail":{"status":"missing_permissions","message":"The API key … missing the permission voices_read …"}}
//
// We surface this back to the client verbatim so the user sees exactly which
// scope is missing instead of a generic "Invalid API key".
type elevenLabsErrorBody struct {
	Detail struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"detail"`
}

// decodeElevenLabsError best-effort decodes the upstream error body. The
// caller is responsible for closing the body and for handling any zero-value
// fields. Failures are silent because the goal is best-effort enrichment.
func decodeElevenLabsError(body io.Reader) elevenLabsErrorBody {
	var ev elevenLabsErrorBody
	_ = json.NewDecoder(body).Decode(&ev)
	return ev
}

// elevenLabsAuthErrorResponse turns a 401/403 from ElevenLabs into the JSON
// body the frontend should toast. It distinguishes between truly-invalid keys
// and valid-but-scoped keys (the most common cause of confusion). The HTTP
// status is intentionally 422 — NOT 401 — because client.ts treats 401 as a
// session-auth failure and force-logs out the user (which is wrong here: the
// user's session is fine; only the ElevenLabs key is rejected).
func elevenLabsAuthErrorResponse(ev elevenLabsErrorBody) gin.H {
	switch ev.Detail.Status {
	case "missing_permissions":
		// Keep the upstream message — it names the missing scope.
		msg := ev.Detail.Message
		if msg == "" {
			msg = "API key is missing required permissions."
		}
		return gin.H{
			"error": msg + " Regenerate the key in ElevenLabs with voices_read, text_to_speech and user_read enabled.",
			"code":  "missing_permissions",
		}
	case "":
		return gin.H{"error": "Invalid API key", "code": "invalid_key"}
	default:
		msg := ev.Detail.Message
		if msg == "" {
			msg = "ElevenLabs rejected the API key."
		}
		return gin.H{"error": msg, "code": ev.Detail.Status}
	}
}

// ---- HandleSaveTTSConfig (D-11) --------------------------------------------

// HandleSaveTTSConfig handles POST /:id/tts-config. Body {api_key, voice_id}.
// The api_key is encrypted server-side before persistence (T-13-01).
func (h *TTSHandler) HandleSaveTTSConfig(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	var req struct {
		APIKey  string `json:"api_key"`
		VoiceID string `json:"voice_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
		return
	}
	if strings.TrimSpace(req.VoiceID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "voice_id is required"})
		return
	}

	encrypted, err := h.cipher.EncryptString(req.APIKey)
	if err != nil {
		h.logger.Error("tts config save: encrypt failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to secure api key"})
		return
	}

	if _, err := h.repo.CreateOrUpdate(c.Request.Context(), overlayID, []byte(encrypted), req.VoiceID); err != nil {
		h.logger.Error("tts config save: persist failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save tts config"})
		return
	}

	h.logger.Info("tts config saved", zap.String("overlay_id", overlayID))
	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

// ---- HandleSaveVoice (Issue #276) ------------------------------------------

// HandleSaveVoice handles PATCH /:id/tts-config/voice. Body {voice_id}.
// Updates only the voice_id column without touching the encrypted_api_key or
// tts_signing_secret. Lets users switch voices after the key is already
// saved (where POST /:id/tts-config requires the api_key in the body and
// the UI hides the "Save key" button once a key exists).
func (h *TTSHandler) HandleSaveVoice(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	var req struct {
		VoiceID string `json:"voice_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	voiceID := strings.TrimSpace(req.VoiceID)
	if voiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "voice_id is required"})
		return
	}

	if err := h.repo.UpdateVoiceID(c.Request.Context(), overlayID, voiceID); err != nil {
		if errors.Is(err, repository.ErrTTSConfigNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tts config not found"})
			return
		}
		h.logger.Error("tts voice update: persist failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save voice"})
		return
	}

	h.logger.Info("tts voice updated",
		zap.String("overlay_id", overlayID), zap.String("voice_id", voiceID))
	c.JSON(http.StatusOK, gin.H{"status": "saved", "voice_id": voiceID})
}

// ---- HandleDeleteTTSConfig (D-12) ------------------------------------------

// HandleDeleteTTSConfig handles DELETE /:id/tts-config.
func (h *TTSHandler) HandleDeleteTTSConfig(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	err := h.repo.Delete(c.Request.Context(), overlayID)
	if err != nil && !errors.Is(err, repository.ErrTTSConfigNotFound) {
		h.logger.Error("tts config delete: failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tts config"})
		return
	}

	h.logger.Info("tts config deleted", zap.String("overlay_id", overlayID))
	c.Status(http.StatusNoContent)
}

// ---- HandleRotateToken (D-13) ----------------------------------------------

// HandleRotateToken handles POST /:id/tts-config/rotate-token. Rotates the
// per-overlay signing secret (invalidating every previously-issued
// tts_token) and returns a fresh obs_url.
func (h *TTSHandler) HandleRotateToken(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	newSecret, err := h.repo.RotateSigningSecret(c.Request.Context(), overlayID)
	if err != nil {
		if errors.Is(err, repository.ErrTTSConfigNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tts config not found"})
			return
		}
		h.logger.Error("tts config rotate: failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate token"})
		return
	}

	token, err := ttspkg.SignOverlayToken(overlayID, newSecret)
	if err != nil {
		h.logger.Error("tts config rotate: sign failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mint token"})
		return
	}

	h.logger.Info("tts signing secret rotated", zap.String("overlay_id", overlayID))
	c.JSON(http.StatusOK, gin.H{"obs_url": h.buildOBSURL(overlayID, token)})
}

// ---- HandleGetVoices (D-14) ------------------------------------------------

// HandleGetVoices handles GET /:id/tts-voices by proxying to ElevenLabs'
// voice list using the decrypted stored api key.
func (h *TTSHandler) HandleGetVoices(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	cfg, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		if errors.Is(err, repository.ErrTTSConfigNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ElevenLabs not configured"})
			return
		}
		h.logger.Error("tts voices: load failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tts config"})
		return
	}

	apiKey, err := h.cipher.DecryptString(string(cfg.EncryptedAPIKey))
	if err != nil {
		h.logger.Error("tts voices: decrypt failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt stored key"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
		h.elevenLabsBaseURL+"/v1/voices", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Warn("tts voices: upstream error",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs request failed"})
		return
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		ev := decodeElevenLabsError(resp.Body)
		c.JSON(http.StatusUnprocessableEntity, elevenLabsAuthErrorResponse(ev))
		return
	case resp.StatusCode == http.StatusTooManyRequests:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate-limited — try again in a minute"})
		return
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs returned non-200"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// ---- HandleGetVoicesPreview (chicken-and-egg break) ------------------------

// HandleGetVoicesPreview handles POST /:id/tts-voices/preview. Body
// {api_key}. The supplied key is used for a one-shot ElevenLabs /v1/voices
// proxy call and is NOT persisted. This breaks the chicken-and-egg between
// HandleSaveTTSConfig (which requires voice_id) and HandleGetVoices (which
// requires a saved key) — clients can populate the voice picker before the
// first save.
func (h *TTSHandler) HandleGetVoicesPreview(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
		return
	}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
		h.elevenLabsBaseURL+"/v1/voices", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	upstreamReq.Header.Set("xi-api-key", apiKey)
	upstreamReq.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		h.logger.Warn("tts voices preview: upstream error",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs request failed"})
		return
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		ev := decodeElevenLabsError(resp.Body)
		c.JSON(http.StatusUnprocessableEntity, elevenLabsAuthErrorResponse(ev))
		return
	case resp.StatusCode == http.StatusTooManyRequests:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate-limited — try again in a minute"})
		return
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs returned non-200"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// ---- HandleTestKey (D-15) --------------------------------------------------

// HandleTestKey handles POST /:id/tts-config/test. It first validates the
// key via GET /v1/user/subscription, then (on success) streams a 2-second
// sample back to the browser with audio/mpeg Content-Type and
// x-characters-remaining / x-characters-limit headers.
func (h *TTSHandler) HandleTestKey(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	cfg, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		if errors.Is(err, repository.ErrTTSConfigNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ElevenLabs not configured"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tts config"})
		return
	}

	apiKey, err := h.cipher.DecryptString(string(cfg.EncryptedAPIKey))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt stored key"})
		return
	}

	// Step 1: GET /v1/user/subscription to validate the key + fetch quota.
	subReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
		h.elevenLabsBaseURL+"/v1/user/subscription", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	subReq.Header.Set("xi-api-key", apiKey)
	subReq.Header.Set("Accept", "application/json")

	subResp, err := h.httpClient.Do(subReq)
	if err != nil {
		h.logger.Warn("tts test: upstream error",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs service unavailable"})
		return
	}
	defer subResp.Body.Close()

	switch subResp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusUnauthorized, http.StatusForbidden:
		ev := decodeElevenLabsError(subResp.Body)
		c.JSON(http.StatusUnprocessableEntity, elevenLabsAuthErrorResponse(ev))
		return
	case http.StatusTooManyRequests:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate-limited — try again in a minute"})
		return
	default:
		if subResp.StatusCode >= 500 {
			c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs service unavailable"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs returned unexpected status"})
		return
	}

	// Parse quota fields (best effort — if the shape changes we still
	// succeed, just with zero headers).
	var quota struct {
		CharacterCount int `json:"character_count"`
		CharacterLimit int `json:"character_limit"`
	}
	if err := json.NewDecoder(subResp.Body).Decode(&quota); err != nil {
		// Non-fatal; carry on without headers.
		h.logger.Debug("tts test: quota decode skipped",
			zap.String("overlay_id", overlayID), zap.Error(err))
	}
	remaining := quota.CharacterLimit - quota.CharacterCount
	if remaining < 0 {
		remaining = 0
	}

	// Step 2: POST /v1/text-to-speech/{voice}/stream with the sample text.
	body, _ := json.Marshal(map[string]interface{}{
		"text":     ttsSampleText,
		"model_id": ttsModel,
	})
	ttsURL := fmt.Sprintf("%s/v1/text-to-speech/%s/stream",
		h.elevenLabsBaseURL, cfg.VoiceID)
	ttsReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		ttsURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build tts request"})
		return
	}
	ttsReq.Header.Set("xi-api-key", apiKey)
	ttsReq.Header.Set("Content-Type", "application/json")
	ttsReq.Header.Set("Accept", "audio/mpeg")

	ttsResp, err := h.httpClient.Do(ttsReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs service unavailable"})
		return
	}
	defer ttsResp.Body.Close()

	if ttsResp.StatusCode < 200 || ttsResp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs synthesis failed"})
		return
	}

	c.Writer.Header().Set("Content-Type", "audio/mpeg")
	if quota.CharacterLimit > 0 {
		c.Writer.Header().Set("x-characters-remaining", fmt.Sprintf("%d", remaining))
		c.Writer.Header().Set("x-characters-limit", fmt.Sprintf("%d", quota.CharacterLimit))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, ttsResp.Body)
}

// ---- HandleTTS (D-16) ------------------------------------------------------

// HandleTTS handles POST /:id/tts?text=...&voice=...&tts_token=... — the
// streaming proxy used by OBS browser sources. tts_token JWT auth; no user
// JWT. Streams upstream audio/mpeg back directly; client disconnect
// propagates to ElevenLabs via request context (T-13-10).
//
// Note: no premium gate on this endpoint. The graceful-permanence contract
// is that a downgraded-to-free user keeps TTS until they (or an admin)
// rotate the signing secret. This prevents overlay audio from cutting out
// mid-stream when a subscription lapses.
func (h *TTSHandler) HandleTTS(c *gin.Context) {
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	text := c.Query("text")
	if text == "" {
		// Allow text in JSON body as a fallback.
		var req struct {
			Text string `json:"text"`
		}
		_ = c.ShouldBindJSON(&req)
		text = req.Text
	}
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}
	if len(text) > maxTTSText {
		text = text[:maxTTSText]
	}

	// Rate limit BEFORE expensive work (key decrypt, JWT verify).
	if !h.checkRateLimit(overlayID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}

	cfg, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		// Uniform 401 so the response doesn't disclose whether a given
		// overlay has a TTS config.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ttsToken := c.Query("tts_token")
	if err := ttspkg.VerifyOverlayToken(ttsToken, overlayID, cfg.SigningSecret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	apiKey, err := h.cipher.DecryptString(string(cfg.EncryptedAPIKey))
	if err != nil {
		h.logger.Error("tts: decrypt failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	voiceID := c.Query("voice")
	if voiceID == "" {
		voiceID = cfg.VoiceID
	}

	body, _ := json.Marshal(map[string]interface{}{
		"text":     text,
		"model_id": ttsModel,
	})
	ttsURL := fmt.Sprintf("%s/v1/text-to-speech/%s/stream", h.elevenLabsBaseURL, voiceID)
	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		ttsURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	upstreamReq.Header.Set("xi-api-key", apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "audio/mpeg")

	// Streaming POST: no Timeout on the client for this call — we rely on
	// context cancellation (Pitfall 7). Use an unbounded http.Client just
	// for streaming.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(upstreamReq)
	if err != nil {
		h.logger.Warn("tts: upstream error",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "ElevenLabs request failed"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Forward status + body so the frontend can switch to the session-
		// wide Web-Speech fallback on any upstream error (D-38).
		c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		c.Status(resp.StatusCode)
		_, _ = io.Copy(c.Writer, resp.Body)
		return
	}

	c.Writer.Header().Set("Content-Type", "audio/mpeg")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// ---- HandleGetTTSConfig (Research Open Question 3) -------------------------

// HandleGetTTSConfig handles GET /:id/tts-config. Returns
//
//   {"has_elevenlabs_config": bool, "voice_id": "...", "obs_url": "..."}
//
// Never includes api_key, encrypted_api_key, or tts_signing_secret — T-13-09.
// The endpoint does NOT require premium: a user whose subscription has
// lapsed still needs to see the current state (graceful-downgrade
// visibility per the plan).
func (h *TTSHandler) HandleGetTTSConfig(c *gin.Context) {
	_, overlayID, ok := h.checkOwnership(c)
	if !ok {
		return
	}

	cfg, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		if errors.Is(err, repository.ErrTTSConfigNotFound) {
			c.JSON(http.StatusOK, gin.H{"has_elevenlabs_config": false})
			return
		}
		h.logger.Error("tts config get: load failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tts config"})
		return
	}

	token, err := ttspkg.SignOverlayToken(overlayID, cfg.SigningSecret)
	if err != nil {
		h.logger.Error("tts config get: sign failed",
			zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mint token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_elevenlabs_config": true,
		"voice_id":              cfg.VoiceID,
		"obs_url":               h.buildOBSURL(overlayID, token),
	})
}
