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
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestSignAndVerifyRequest(t *testing.T) {
	signer := NewSigner("test-service", "test-secret", zap.NewNop())

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

	secret := "test-secret"
	signer := NewSigner("test-service", secret, zap.NewNop())
	verifier := NewSigner("verifier-service", secret, zap.NewNop())

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

	verifier := NewSigner("verifier", "secret", zap.NewNop())

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

	verifier := NewSigner("verifier", "correct-secret", zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Sign with wrong secret
	wrongSigner := NewSigner("test-service", "wrong-secret", zap.NewNop())
	req := httptest.NewRequest("GET", "/test", nil)
	wrongSigner.SignRequest(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid signature")
}

func TestVerifyMiddleware_ExpiredRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	secret := "test-secret"
	signer := NewSigner("test-service", secret, zap.NewNop())
	verifier := NewSigner("verifier", secret, zap.NewNop())

	router := gin.New()
	router.Use(verifier.VerifyMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Create request with old timestamp
	req := httptest.NewRequest("GET", "/test", nil)
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	signature := signer.computeSignature("GET", "/test", oldTimestamp, nil)

	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(oldTimestamp, 10))
	req.Header.Set(HeaderService, "test-service")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "request too old")
}

func TestComputeSignature_Consistency(t *testing.T) {
	signer := NewSigner("test", "secret", zap.NewNop())

	body := []byte("test body")
	timestamp := time.Now().Unix()

	// Same inputs should produce same signature
	sig1 := signer.computeSignature("POST", "/api/test", timestamp, body)
	sig2 := signer.computeSignature("POST", "/api/test", timestamp, body)

	assert.Equal(t, sig1, sig2)

	// Different inputs should produce different signatures
	sig3 := signer.computeSignature("GET", "/api/test", timestamp, body)
	assert.NotEqual(t, sig1, sig3)

	sig4 := signer.computeSignature("POST", "/api/different", timestamp, body)
	assert.NotEqual(t, sig1, sig4)

	sig5 := signer.computeSignature("POST", "/api/test", timestamp+1, body)
	assert.NotEqual(t, sig1, sig5)

	sig6 := signer.computeSignature("POST", "/api/test", timestamp, []byte("different body"))
	assert.NotEqual(t, sig1, sig6)
}

func TestVerifySignature(t *testing.T) {
	signer := NewSigner("test", "secret", zap.NewNop())

	body := []byte("test data")
	timestamp := time.Now().Unix()
	signature := signer.computeSignature("POST", "/api/test", timestamp, body)

	// Valid signature
	err := signer.VerifySignature("POST", "/api/test", timestamp, body, signature)
	assert.NoError(t, err)

	// Invalid signature
	err = signer.VerifySignature("POST", "/api/test", timestamp, body, "invalid")
	assert.Equal(t, ErrInvalidSignature, err)

	// Expired timestamp
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	oldSignature := signer.computeSignature("POST", "/api/test", oldTimestamp, body)
	err = signer.VerifySignature("POST", "/api/test", oldTimestamp, body, oldSignature)
	assert.Equal(t, ErrRequestTooOld, err)
}

func TestSigningTransport(t *testing.T) {
	// Create test server that verifies signatures
	secret := "test-secret"
	verifier := NewSigner("server", secret, zap.NewNop())

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
	client := NewSigningClient("test-client", secret, zap.NewNop())

	// Make request
	resp, err := client.Get(server.URL + "/test")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "test-client")
}

func TestSigningTransport_WithBody(t *testing.T) {
	secret := "test-secret"
	verifier := NewSigner("server", secret, zap.NewNop())

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
	client := NewSigningClient("test-client", secret, zap.NewNop())

	// Make POST request with body
	body := bytes.NewBufferString(`{"message":"hello"}`)
	resp, err := client.Post(server.URL+"/test", "application/json", body)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(respBody), "hello")
}
