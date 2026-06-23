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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"net"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// Header names for request signing
	HeaderSignature = "X-Service-Signature"
	HeaderTimestamp = "X-Service-Timestamp"
	HeaderService   = "X-Service-Name"

	// Maximum age of signed requests (prevents replay attacks)
	MaxRequestAge = 5 * time.Minute
)

var (
	ErrMissingSignature = errors.New("missing signature header")
	ErrMissingTimestamp = errors.New("missing timestamp header")
	ErrMissingService   = errors.New("missing service name header")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrRequestTooOld    = errors.New("request timestamp too old")
	ErrRequestInFuture  = errors.New("request timestamp is too far in the future")
	ErrInvalidTimestamp = errors.New("invalid timestamp format")
	ErrSecretTooShort   = errors.New("signing secret must be at least 32 bytes")
)

// MaxFutureSkew is the maximum tolerance for request timestamps slightly ahead
// of the server clock. Timestamps further in the future are rejected to close
// the future-timestamp replay window (audit M4).
const MaxFutureSkew = time.Minute

// Signer signs HTTP requests with HMAC-SHA256
type Signer struct {
	serviceName string
	secret      []byte
	logger      *zap.Logger
}

// NewSigner creates a new request signer. Returns ErrSecretTooShort if the
// secret is fewer than 32 bytes (audit L1).
func NewSigner(serviceName string, secret string, logger *zap.Logger) (*Signer, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrSecretTooShort, len(secret))
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Signer{
		serviceName: serviceName,
		secret:      []byte(secret),
		logger:      logger,
	}, nil
}

// SignRequest adds signature headers to an HTTP request
func (s *Signer) SignRequest(req *http.Request) error {
	// Get current timestamp
	timestamp := time.Now().Unix()

	// Read body for signing (if present)
	var body []byte
	var err error
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("read request body: %w", err)
		}
		// Reset body for actual request
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Create signature (includes query params + service name, audit M5)
	signature := s.computeSignature(req.Method, req.URL.Path, req.URL.RawQuery, s.serviceName, timestamp, body)

	// Add headers
	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(HeaderService, s.serviceName)

	return nil
}

// VerifyMiddleware returns a Gin middleware that verifies request signatures
func (s *Signer) VerifyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract headers
		signature := c.GetHeader(HeaderSignature)
		timestampStr := c.GetHeader(HeaderTimestamp)
		serviceName := c.GetHeader(HeaderService)

		// Validate headers presence
		if signature == "" {
			s.logger.Warn("Request missing signature header",
				zap.String("path", c.Request.URL.Path),
				zap.String("remote", anonymizeIP(c.ClientIP())))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature"})
			c.Abort()
			return
		}

		if timestampStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing timestamp"})
			c.Abort()
			return
		}

		if serviceName == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing service name"})
			c.Abort()
			return
		}

		// Parse timestamp
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid timestamp"})
			c.Abort()
			return
		}

		// Check request age (prevent replay attacks, audit M4)
		requestTime := time.Unix(timestamp, 0)
		age := time.Since(requestTime)
		if age > MaxRequestAge {
			s.logger.Warn("Request timestamp too old",
				zap.String("service", serviceName),
				zap.Time("timestamp", requestTime),
				zap.Duration("age", age))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "request too old"})
			c.Abort()
			return
		}
		// Reject future-dated timestamps: time.Since is negative for future ts,
		// so without this check a future-dated request is accepted indefinitely
		// until it "ages in" (audit M4).
		if time.Until(requestTime) > MaxFutureSkew {
			s.logger.Warn("Request timestamp too far in the future",
				zap.String("service", serviceName),
				zap.Time("timestamp", requestTime),
				zap.Duration("future_skew", time.Until(requestTime)))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "request timestamp in the future"})
			c.Abort()
			return
		}

		// Read body for verification
		var body []byte
		if c.Request.Body != nil {
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				s.logger.Error("Failed to read request body", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				c.Abort()
				return
			}
			// Reset body for handlers
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}

		// Compute expected signature (includes query params + service name, audit M5)
		expectedSignature := s.computeSignature(c.Request.Method, c.Request.URL.Path, c.Request.URL.RawQuery, serviceName, timestamp, body)

		// Verify signature (constant-time comparison)
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			s.logger.Warn("Invalid request signature",
				zap.String("service", serviceName),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			c.Abort()
			return
		}

		// Add service name to context for handlers
		c.Set("service_name", serviceName)

		s.logger.Debug("Request signature verified",
			zap.String("service", serviceName),
			zap.String("path", c.Request.URL.Path))

		c.Next()
	}
}

// computeSignature creates HMAC-SHA256 signature.
// Format: HMAC-SHA256(secret, "method|path|query|service|timestamp|body_hash")
//
// The query string and service name are included to prevent tampering with
// query parameters or spoofing a different service identity (audit M5).
func (s *Signer) computeSignature(method, path, rawQuery, serviceName string, timestamp int64, body []byte) string {
	// Hash body
	bodyHash := sha256.Sum256(body)

	// Create message to sign
	message := fmt.Sprintf("%s|%s|%s|%s|%d|%s",
		method,
		path,
		rawQuery,
		serviceName,
		timestamp,
		hex.EncodeToString(bodyHash[:]))

	// Compute HMAC
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(message))

	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies a signature without using middleware. serviceName
// and rawQuery must match what the signer used (audit M5). Future-dated
// timestamps are rejected (audit M4).
func (s *Signer) VerifySignature(method, path, rawQuery, serviceName string, timestamp int64, body []byte, signature string) error {
	// Check timestamp
	requestTime := time.Unix(timestamp, 0)
	age := time.Since(requestTime)
	if age > MaxRequestAge {
		return ErrRequestTooOld
	}
	if time.Until(requestTime) > MaxFutureSkew {
		return ErrRequestInFuture
	}

	// Compute expected signature
	expectedSignature := s.computeSignature(method, path, rawQuery, serviceName, timestamp, body)

	// Verify
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return ErrInvalidSignature
	}

	return nil
}

// HTTPClient returns an http.Client that automatically signs requests
type SigningTransport struct {
	base   http.RoundTripper
	signer *Signer
}

// NewSigningTransport creates a transport that signs all requests.
// The signer must be non-nil.
func NewSigningTransport(base http.RoundTripper, signer *Signer) *SigningTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &SigningTransport{
		base:   base,
		signer: signer,
	}
}

// RoundTrip implements http.RoundTripper
func (t *SigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Sign request
	if err := t.signer.SignRequest(req); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	// Execute request
	return t.base.RoundTrip(req)
}

// NewSigningClient creates an HTTP client that automatically signs all requests.
// Returns an error if the secret is fewer than 32 bytes (audit L1).
func NewSigningClient(serviceName, secret string, logger *zap.Logger) (*http.Client, error) {
	signer, err := NewSigner(serviceName, secret, logger)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: NewSigningTransport(nil, signer),
		Timeout:   30 * time.Second,
	}, nil
}

// anonymizeIP truncates the last octet (IPv4) or last 80 bits (IPv6) for
// DSGVO-compliant log output.
func anonymizeIP(raw string) string {
	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}
	if v4 := ip.To4(); v4 != nil {
		v4[3] = 0
		return v4.String()
	}
	v6 := ip.To16()
	for i := 6; i < 16; i++ {
		v6[i] = 0
	}
	return v6.String()
}
