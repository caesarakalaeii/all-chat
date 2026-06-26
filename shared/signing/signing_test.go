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

package signing

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// 32-byte test secret that satisfies the minimum-length check (audit L1).
const testSecret = "0123456789abcdef0123456789abcdef"

func mustNewSigner(t *testing.T, serviceName, secret string, logger *zap.Logger) *Signer {
	t.Helper()
	s, err := NewSigner(serviceName, secret, logger)
	require.NoError(t, err)
	return s
}

func TestNewSigner_RejectsShortSecret(t *testing.T) {
	_, err := NewSigner("svc", "short", zap.NewNop())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSecretTooShort)
}

func TestNewSigner_AcceptsMinLengthSecret(t *testing.T) {
	s, err := NewSigner("svc", testSecret, zap.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestSignAndVerifyRequest(t *testing.T) {
	signer := mustNewSigner(t, "test-service", testSecret, zap.NewNop())

	// Create request
	req := httptest.NewRequest("POST", "/api/test", bytes.NewBufferString(`{"key":"value"}`))

	// Sign request
	err := signer.SignRequest(req)
	assert.NoError(t, err)

	// Verify headers are present
	assert.NotEmpty(t, req.Header.Get(HeaderSignature))
	assert.NotEmpty(t, req.Header.Get(HeaderTimestamp))
	assert.Equal(t, "test-service", req.Header.Get(HeaderService))
}

func TestVerifyMiddleware_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	signer := mustNewSigner(t, "test-service", testSecret, zap.NewNop())
	verifier := mustNewSigner(t, "verifier-service", testSecret, zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.POST("/test", func(c *gin.Context) {
		serviceName, _ := c.Get("service_name")
		c.JSON(200, gin.H{"service": serviceName})
	})

	// Create and sign request
	body := []byte(`{"test":"data"}`)
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(body))
	signer.SignRequest(req)

	// Execute request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "test-service")
}

func TestVerifyMiddleware_MissingSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifier := mustNewSigner(t, "verifier", testSecret, zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Request without signature
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "missing signature")
}

func TestVerifyMiddleware_InvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifier := mustNewSigner(t, "verifier", testSecret, zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Sign with wrong secret
	wrongSigner := mustNewSigner(t, "test-service", "0123456789abcdef0123456789abcdef_wrong", zap.NewNop())
	req := httptest.NewRequest("GET", "/test", nil)
	wrongSigner.SignRequest(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid signature")
}

func TestVerifyMiddleware_ExpiredRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	signer := mustNewSigner(t, "test-service", testSecret, zap.NewNop())
	verifier := mustNewSigner(t, "verifier", testSecret, zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Create request with old timestamp
	req := httptest.NewRequest("GET", "/test", nil)
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	signature := signer.computeSignature("GET", "/test", "", "test-service", oldTimestamp, nil)

	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(oldTimestamp, 10))
	req.Header.Set(HeaderService, "test-service")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "request too old")
}

func TestVerifyMiddleware_FutureTimestampRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	signer := mustNewSigner(t, "test-service", testSecret, zap.NewNop())
	verifier := mustNewSigner(t, "verifier", testSecret, zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Create request with future timestamp (audit M4)
	req := httptest.NewRequest("GET", "/test", nil)
	futureTimestamp := time.Now().Add(10 * time.Minute).Unix()
	signature := signer.computeSignature("GET", "/test", "", "test-service", futureTimestamp, nil)

	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(futureTimestamp, 10))
	req.Header.Set(HeaderService, "test-service")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "future")
}

func TestComputeSignature_Consistency(t *testing.T) {
	signer := mustNewSigner(t, "test", testSecret, zap.NewNop())

	body := []byte("test body")
	timestamp := time.Now().Unix()
	query := "foo=bar"
	svc := "test"

	// Same inputs should produce same signature
	sig1 := signer.computeSignature("POST", "/api/test", query, svc, timestamp, body)
	sig2 := signer.computeSignature("POST", "/api/test", query, svc, timestamp, body)

	assert.Equal(t, sig1, sig2)

	// Different inputs should produce different signatures
	sig3 := signer.computeSignature("GET", "/api/test", query, svc, timestamp, body)
	assert.NotEqual(t, sig1, sig3)

	sig4 := signer.computeSignature("POST", "/api/different", query, svc, timestamp, body)
	assert.NotEqual(t, sig1, sig4)

	sig5 := signer.computeSignature("POST", "/api/test", query, svc, timestamp+1, body)
	assert.NotEqual(t, sig1, sig5)

	sig6 := signer.computeSignature("POST", "/api/test", query, svc, timestamp, []byte("different body"))
	assert.NotEqual(t, sig1, sig6)

	// Different query produces different signature (audit M5)
	sig7 := signer.computeSignature("POST", "/api/test", "different=query", svc, timestamp, body)
	assert.NotEqual(t, sig1, sig7)

	// Different service name produces different signature (audit M5)
	sig8 := signer.computeSignature("POST", "/api/test", query, "other-service", timestamp, body)
	assert.NotEqual(t, sig1, sig8)
}

func TestVerifySignature(t *testing.T) {
	signer := mustNewSigner(t, "test", testSecret, zap.NewNop())

	body := []byte("test data")
	timestamp := time.Now().Unix()
	query := ""
	svc := "test"
	signature := signer.computeSignature("POST", "/api/test", query, svc, timestamp, body)

	// Valid signature
	err := signer.VerifySignature("POST", "/api/test", query, svc, timestamp, body, signature)
	assert.NoError(t, err)

	// Invalid signature
	err = signer.VerifySignature("POST", "/api/test", query, svc, timestamp, body, "invalid")
	assert.Equal(t, ErrInvalidSignature, err)

	// Expired timestamp
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	oldSignature := signer.computeSignature("POST", "/api/test", query, svc, oldTimestamp, body)
	err = signer.VerifySignature("POST", "/api/test", query, svc, oldTimestamp, body, oldSignature)
	assert.Equal(t, ErrRequestTooOld, err)

	// Future timestamp (audit M4)
	futureTimestamp := time.Now().Add(10 * time.Minute).Unix()
	futureSignature := signer.computeSignature("POST", "/api/test", query, svc, futureTimestamp, body)
	err = signer.VerifySignature("POST", "/api/test", query, svc, futureTimestamp, body, futureSignature)
	assert.Equal(t, ErrRequestInFuture, err)
}

func TestSigningTransport(t *testing.T) {
	// Create test server that verifies signatures
	verifier := mustNewSigner(t, "server", testSecret, zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		serviceName, _ := c.Get("service_name")
		c.JSON(200, gin.H{"service": serviceName})
	})

	server := httptest.NewServer(router)
	defer server.Close()

	// Create client with signing transport
	client, err := NewSigningClient("test-client", testSecret, zap.NewNop())
	require.NoError(t, err)

	// Make request
	resp, err := client.Get(server.URL + "/test")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "test-client")
}

func TestSigningTransport_WithBody(t *testing.T) {
	verifier := mustNewSigner(t, "server", testSecret, zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.POST("/test", func(c *gin.Context) {
		var data map[string]string
		c.BindJSON(&data)
		c.JSON(200, gin.H{"received": data["message"]})
	})

	server := httptest.NewServer(router)
	defer server.Close()

	// Create client with signing
	client, err := NewSigningClient("test-client", testSecret, zap.NewNop())
	require.NoError(t, err)

	// Make POST request with body
	body := bytes.NewBufferString(`{"message":"hello"}`)
	resp, err := client.Post(server.URL+"/test", "application/json", body)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(respBody), "hello")
}

// TestComputeSignature_NoCrossFieldCollision guards audit #26: the per-field
// fixed-width framing must make field-boundary shifts impossible. Under the old
// "|"-delimited format, (path="/a", query="b|x") and (path="/a|b", query="x")
// hashed identically. They must now diverge.
func TestComputeSignature_NoCrossFieldCollision(t *testing.T) {
	s := mustNewSigner(t, "svc", testSecret, zap.NewNop())
	const ts = int64(1000)
	a := s.computeSignature("POST", "/a", "b|x", "y", ts, nil)
	b := s.computeSignature("POST", "/a|b", "x", "y", ts, nil)
	assert.NotEqual(t, a, b, "path/query boundary shift must not collide")

	// Same shift between rawQuery and serviceName must also diverge.
	c := s.computeSignature("POST", "/p", "q", "svc-a", ts, nil)
	d := s.computeSignature("POST", "/p", "q|svc", "a", ts, nil)
	assert.NotEqual(t, c, d, "query/service boundary shift must not collide")
}
