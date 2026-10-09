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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/media-service/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testUserID = "user-1"
const otherUserID = "user-2"

// mockRegistry is an in-memory MediaRegistry. The *Err fields inject a
// failure into the corresponding method so the handler's 500 branches can
// be exercised without a database.
type mockRegistry struct {
	objects   []models.MediaObject
	createErr error
	countErr  error
	listErr   error
	deleteErr error
}

func (m *mockRegistry) Create(_ context.Context, obj *models.MediaObject) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.objects = append(m.objects, *obj)
	return nil
}

func (m *mockRegistry) CountByUser(_ context.Context, userID string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	n := 0
	for _, o := range m.objects {
		if o.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (m *mockRegistry) ListByUser(_ context.Context, userID string) ([]models.MediaObject, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []models.MediaObject
	for _, o := range m.objects {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *mockRegistry) DeleteByOwner(_ context.Context, userID, objectKey string) (bool, error) {
	if m.deleteErr != nil {
		return false, m.deleteErr
	}
	for i, o := range m.objects {
		if o.UserID == userID && o.ObjectKey == objectKey {
			m.objects = append(m.objects[:i], m.objects[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// mockStore is an in-memory ObjectStore that records what the handler asked
// it to do.
type mockStore struct {
	available bool

	presignURL  string
	presignErr  error
	removedKeys []string
	removeErr   error

	presignedKey  string
	presignExpiry time.Duration
}

func (m *mockStore) Available() bool { return m.available }

func (m *mockStore) PresignPut(_ context.Context, objectKey string, expiry time.Duration) (string, error) {
	if m.presignErr != nil {
		return "", m.presignErr
	}
	m.presignedKey = objectKey
	m.presignExpiry = expiry
	return m.presignURL, nil
}

func (m *mockStore) Remove(_ context.Context, objectKey string) error {
	m.removedKeys = append(m.removedKeys, objectKey)
	if m.removeErr != nil {
		return m.removeErr
	}
	return nil
}

// mediaRouter wires the media routes behind the same shape cmd/main.go
// registers them, injecting a user_id like JWTAuth does.
func mediaRouter(reg MediaRegistry, store ObjectStore, cfg MediaConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewMediaHandler(reg, store, cfg, zap.NewNop())
	auth := func(c *gin.Context) {
		c.Set("user_id", testUserID)
		c.Next()
	}
	media := r.Group("/api/v1/media", auth, h.RequireStore)
	media.POST("/presign", h.Presign)
	media.GET("", h.List)
	media.DELETE("/*object_key", h.Delete)
	return r
}

func testConfig() MediaConfig {
	return MediaConfig{
		PublicBaseURL:     "https://media.allch.at",
		PresignExpiry:     5 * time.Minute,
		MaxObjectsPerUser: 2,
	}
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestPresign_CreatesRegistryRowAndReturnsURLs(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true, presignURL: "https://minio.local/upload?sig=1"}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"airhorn.mp3","content_type":"audio/mpeg","size":1234}`)

	require.Equal(t, http.StatusCreated, w.Code)
	body := decode(t, w)

	objectKey, _ := body["object_key"].(string)
	assert.Equal(t, "https://minio.local/upload?sig=1", body["upload_url"])
	assert.Equal(t, "https://media.allch.at/"+objectKey, body["public_url"])

	// Key scheme: {user_id}/{uuid}/{filename} — the randomized middle
	// segment makes the public URL unguessable (ADR-0064).
	parts := strings.Split(objectKey, "/")
	require.Len(t, parts, 3, "object_key must be user/uuid/filename, got %q", objectKey)
	assert.Equal(t, testUserID, parts[0])
	assert.NotEmpty(t, parts[1], "uuid segment must not be empty")
	assert.Equal(t, "airhorn.mp3", parts[2])

	// The registry row is created at presign time, before the PUT happens.
	require.Len(t, reg.objects, 1)
	row := reg.objects[0]
	assert.Equal(t, objectKey, row.ObjectKey)
	assert.Equal(t, testUserID, row.UserID)
	assert.Equal(t, "airhorn.mp3", row.Filename)
	assert.Equal(t, "audio/mpeg", row.ContentType)
	assert.Equal(t, int64(1234), row.SizeBytes)

	assert.Equal(t, objectKey, store.presignedKey, "presign must target the registry key")
	assert.Equal(t, 5*time.Minute, store.presignExpiry, "presign expiry must come from config")
}

func TestPresign_RejectsUnsupportedContentType(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"payload.exe","content_type":"application/octet-stream","size":100}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported content type")
	assert.Empty(t, reg.objects, "no registry row for a rejected request")
}

func TestPresign_RejectsOversize(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"big.wav","content_type":"audio/wav","size":10485761}`)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Empty(t, reg.objects, "no registry row for a rejected request")
}

func TestPresign_RejectsMissingFields(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true}
	r := mediaRouter(reg, store, testConfig())

	w := doJSON(r, http.MethodPost, "/api/v1/media/presign", `{"content_type":"audio/mpeg","size":10}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSON(r, http.MethodPost, "/api/v1/media/presign", `{"filename":"a.mp3","content_type":"audio/mpeg","size":0}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSON(r, http.MethodPost, "/api/v1/media/presign", `not json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	assert.Empty(t, reg.objects, "no registry row for a rejected request")
}

func TestPresign_SanitizesFilenamePathSeparators(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true, presignURL: "https://minio.local/upload"}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"../../../etc/passwd","content_type":"image/png","size":10}`)

	require.Equal(t, http.StatusCreated, w.Code)
	body := decode(t, w)
	objectKey, _ := body["object_key"].(string)
	assert.NotContains(t, objectKey, "..", "path traversal must not reach the object key")
	assert.True(t, strings.HasSuffix(objectKey, "/passwd"), "filename must be reduced to its base name, got %q", objectKey)
}

func TestPresign_PublicURLAddressesTheObjectItAdvertises(t *testing.T) {
	// public_url is what an alert plays. A filename with URL-reserved
	// characters must not turn it into a URL that resolves somewhere else:
	// "a#b.png" truncates at a fragment, "what?.mp3" at a query string, and
	// a bare "%" is an invalid escape — the object uploads fine and the
	// advertised URL 404s.
	for _, filename := range []string{"a#b.png", "what?.mp3", "100%_horn.mp3", "my sound.mp3"} {
		t.Run(filename, func(t *testing.T) {
			reg := &mockRegistry{}
			store := &mockStore{available: true, presignURL: "https://minio.local/upload"}
			w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
				`{"filename":"`+filename+`","content_type":"image/png","size":10}`)

			require.Equal(t, http.StatusCreated, w.Code)
			body := decode(t, w)
			objectKey, _ := body["object_key"].(string)
			require.True(t, strings.HasSuffix(objectKey, "/"+filename),
				"the object key must keep the raw filename, got %q", objectKey)

			publicURL, _ := body["public_url"].(string)
			parsed, err := url.Parse(publicURL)
			require.NoError(t, err, "public_url must be a parseable URL, got %q", publicURL)
			assert.Empty(t, parsed.Fragment, "public_url must not be truncated by a fragment")
			assert.Empty(t, parsed.RawQuery, "public_url must not be truncated by a query string")
			decodedPath, err := url.PathUnescape(parsed.EscapedPath())
			require.NoError(t, err, "public_url path must be a valid escape")
			assert.Equal(t, "/"+objectKey, decodedPath,
				"fetching public_url must request the uploaded object")
		})
	}
}

func TestPresign_RejectsDotSegmentFilenames(t *testing.T) {
	// A filename that reduces to "." or ".." cannot be carried in a URL:
	// every client normalizes dot segments away before sending, so the
	// public URL would address the wrong path and the DELETE route could
	// never be reached with the key. Reject it instead of storing it.
	// (Names merely containing dots — "a..b.png" — stay legal; see
	// TestDelete_AcceptsKeysPresignCreates.)
	for _, filename := range []string{".", "..", "sub/.."} {
		t.Run(filename, func(t *testing.T) {
			reg := &mockRegistry{}
			store := &mockStore{available: true}
			w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
				`{"filename":"`+filename+`","content_type":"image/png","size":10}`)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Empty(t, reg.objects, "no registry row for a rejected filename")
			assert.Empty(t, store.presignedKey, "no presigned upload for a rejected filename")
		})
	}
}

func TestPresign_EnforcesPerUserQuota(t *testing.T) {
	// Two registered objects against MaxObjectsPerUser: 2 — the quota is
	// checked through the registry count, so seed real rows.
	reg := &mockRegistry{objects: []models.MediaObject{
		{UserID: testUserID, ObjectKey: testUserID + "/uuid1/a.mp3", Filename: "a.mp3", ContentType: "audio/mpeg", SizeBytes: 1},
		{UserID: testUserID, ObjectKey: testUserID + "/uuid2/b.mp3", Filename: "b.mp3", ContentType: "audio/mpeg", SizeBytes: 1},
	}}
	store := &mockStore{available: true, presignURL: "https://minio.local/upload"}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"extra.mp3","content_type":"audio/mpeg","size":10}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "quota")
	assert.Len(t, reg.objects, 2, "quota rejection must not create a registry row")
	assert.Empty(t, store.presignedKey, "quota rejection must not presign an upload")
}

func TestPresign_503WhenStoreUnavailable(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: false}
	r := mediaRouter(reg, store, testConfig())

	w := doJSON(r, http.MethodPost, "/api/v1/media/presign", `{"filename":"a.mp3","content_type":"audio/mpeg","size":10}`)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	w = doJSON(r, http.MethodGet, "/api/v1/media", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	w = doJSON(r, http.MethodDelete, "/api/v1/media/uuid/a.mp3", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	assert.Empty(t, reg.objects)
}

func TestPresign_MinioErrorIsNotRegistered(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true, presignErr: errors.New("minio down")}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"a.mp3","content_type":"audio/mpeg","size":10}`)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Empty(t, reg.objects, "a failed presign must not leave a registry row")
}

func TestList_ReturnsOnlyTheCallerRows(t *testing.T) {
	reg := &mockRegistry{objects: []models.MediaObject{
		{ID: "id-1", UserID: testUserID, ObjectKey: testUserID + "/uuid1/a.mp3", Filename: "a.mp3", ContentType: "audio/mpeg", SizeBytes: 10, CreatedAt: time.Unix(1000, 0).UTC()},
		{ID: "id-2", UserID: otherUserID, ObjectKey: otherUserID + "/uuid2/b.png", Filename: "b.png", ContentType: "image/png", SizeBytes: 20, CreatedAt: time.Unix(2000, 0).UTC()},
	}}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodGet, "/api/v1/media", "")

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Media []models.MediaObject `json:"media"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Media, 1)
	assert.Equal(t, "id-1", body.Media[0].ID)
	assert.Equal(t, testUserID+"/uuid1/a.mp3", body.Media[0].ObjectKey)
}

func TestList_ToleratesObjectsThatNeverArrived(t *testing.T) {
	// A presigned PUT the client never performed still has a registry row
	// (created at presign time); listing must surface it, not error.
	reg := &mockRegistry{objects: []models.MediaObject{
		{ID: "id-orphan", UserID: testUserID, ObjectKey: testUserID + "/uuid/orphan.wav", Filename: "orphan.wav", ContentType: "audio/wav", SizeBytes: 10, CreatedAt: time.Unix(1000, 0).UTC()},
	}}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodGet, "/api/v1/media", "")

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Media []models.MediaObject `json:"media"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body.Media, 1)
}

func TestDelete_RemovesRegistryRowAndBucketObject(t *testing.T) {
	objectKey := testUserID + "/uuid1/a.mp3"
	reg := &mockRegistry{objects: []models.MediaObject{
		{ID: "id-1", UserID: testUserID, ObjectKey: objectKey, Filename: "a.mp3", ContentType: "audio/mpeg", SizeBytes: 10, CreatedAt: time.Unix(1000, 0).UTC()},
	}}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodDelete, "/api/v1/media/"+objectKey, "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, reg.objects, "registry row must be deleted")
	assert.Equal(t, []string{objectKey}, store.removedKeys, "MinIO object must be removed")
}

func TestDelete_OtherUsersObjectIs404(t *testing.T) {
	objectKey := otherUserID + "/uuid2/b.png"
	reg := &mockRegistry{objects: []models.MediaObject{
		{ID: "id-2", UserID: otherUserID, ObjectKey: objectKey, Filename: "b.png", ContentType: "image/png", SizeBytes: 20, CreatedAt: time.Unix(2000, 0).UTC()},
	}}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodDelete, "/api/v1/media/"+objectKey, "")

	// 404, not 403: an object key that is not yours is indistinguishable
	// from one that does not exist.
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Len(t, reg.objects, 1, "the other user's row must survive")
	assert.Empty(t, store.removedKeys, "no MinIO removal without ownership")
}

func TestDelete_MinioErrorStillDeletesRegistryRow(t *testing.T) {
	objectKey := testUserID + "/uuid1/a.mp3"
	reg := &mockRegistry{objects: []models.MediaObject{
		{ID: "id-1", UserID: testUserID, ObjectKey: objectKey, Filename: "a.mp3", ContentType: "audio/mpeg", SizeBytes: 10, CreatedAt: time.Unix(1000, 0).UTC()},
	}}
	store := &mockStore{available: true, removeErr: errors.New("minio down")}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodDelete, "/api/v1/media/"+objectKey, "")

	// The registry row is the source of truth for "user deleted this";
	// a MinIO hiccup must not resurrect it (issue #949: logged, not fatal).
	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, reg.objects, "registry row must be deleted despite the MinIO error")
	assert.Len(t, store.removedKeys, 1, "the removal was still attempted")
}

func TestPresign_RegistryCountErrorIs500(t *testing.T) {
	reg := &mockRegistry{countErr: errors.New("db down")}
	store := &mockStore{available: true, presignURL: "https://minio.local/upload"}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"a.mp3","content_type":"audio/mpeg","size":10}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to check media quota")
	assert.Empty(t, reg.objects, "no registry row without a successful quota check")
	assert.Empty(t, store.presignedKey, "no upload presigned without a successful quota check")
}

func TestPresign_RegistryCreateErrorIs500(t *testing.T) {
	reg := &mockRegistry{createErr: errors.New("db down")}
	store := &mockStore{available: true, presignURL: "https://minio.local/upload"}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodPost, "/api/v1/media/presign",
		`{"filename":"a.mp3","content_type":"audio/mpeg","size":10}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to register media object")
	assert.NotEmpty(t, store.presignedKey, "the presign happened before the failed registry write")
}

func TestList_RegistryErrorIs500(t *testing.T) {
	reg := &mockRegistry{listErr: errors.New("db down")}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodGet, "/api/v1/media", "")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list media")
}

func TestDelete_RegistryErrorIs500(t *testing.T) {
	objectKey := testUserID + "/uuid1/a.mp3"
	reg := &mockRegistry{objects: []models.MediaObject{
		{ID: "id-1", UserID: testUserID, ObjectKey: objectKey, Filename: "a.mp3", ContentType: "audio/mpeg", SizeBytes: 10, CreatedAt: time.Unix(1000, 0).UTC()},
	}, deleteErr: errors.New("db down")}
	store := &mockStore{available: true}
	w := doJSON(mediaRouter(reg, store, testConfig()), http.MethodDelete, "/api/v1/media/"+objectKey, "")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to delete media")
	assert.Len(t, reg.objects, 1, "the registry row must survive a failed delete")
	assert.Empty(t, store.removedKeys, "no MinIO removal when the registry delete failed")
}

func TestDelete_AcceptsKeysPresignCreates(t *testing.T) {
	// Presign's sanitizeFilename keeps filenames that merely contain two
	// dots ("a..b.png" has no path separators), so the key Delete receives
	// back can contain "..". The key is only ever used as an opaque S3 key
	// and an exact-match SQL predicate — there is no traversal to guard —
	// so Delete must accept every key this service hands out, or the row
	// is registered, counts against quota, and can never be deleted.
	reg := &mockRegistry{}
	store := &mockStore{available: true, presignURL: "https://minio.local/upload"}
	r := mediaRouter(reg, store, testConfig())

	w := doJSON(r, http.MethodPost, "/api/v1/media/presign",
		`{"filename":"a..b.png","content_type":"image/png","size":100}`)
	require.Equal(t, http.StatusCreated, w.Code, "presign must accept a..b.png")
	objectKey, _ := decode(t, w)["object_key"].(string)
	parts := strings.Split(objectKey, "/")
	require.Len(t, parts, 3, "object_key must be user/uuid/filename, got %q", objectKey)
	require.Equal(t, testUserID, parts[0])
	require.Equal(t, "a..b.png", parts[2])

	w = doJSON(r, http.MethodDelete, "/api/v1/media/"+objectKey, "")
	require.Equal(t, http.StatusOK, w.Code, "delete must accept the key presign issued")
	assert.Empty(t, reg.objects, "registry row must be deleted")
	assert.Equal(t, []string{objectKey}, store.removedKeys, "MinIO object must be removed")
}

func TestDelete_RejectsEmptyObjectKey(t *testing.T) {
	reg := &mockRegistry{}
	store := &mockStore{available: true}
	r := mediaRouter(reg, store, testConfig())

	// The wildcard hands over a bare slash, which trims to an empty key;
	// that must be rejected before any registry or MinIO call.
	w := doJSON(r, http.MethodDelete, "/api/v1/media/", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	assert.Empty(t, reg.objects)
	assert.Empty(t, store.removedKeys, "no MinIO removal for a rejected key")
}
