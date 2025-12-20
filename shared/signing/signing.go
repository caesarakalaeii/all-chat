package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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
	ErrInvalidTimestamp = errors.New("invalid timestamp format")
)

// Signer signs HTTP requests with HMAC-SHA256
type Signer struct {
	serviceName string
	secret      []byte
	logger      *zap.Logger
}

// NewSigner creates a new request signer
func NewSigner(serviceName string, secret string, logger *zap.Logger) *Signer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Signer{
		serviceName: serviceName,
		secret:      []byte(secret),
		logger:      logger,
	}
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
		req.Body = io.NopCloser(io.Reader(io.MultiReader(io.Reader(io.NopCloser(io.Reader(body))))))
	}

	// Create signature
	signature := s.computeSignature(req.Method, req.URL.Path, timestamp, body)

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
				zap.String("remote", c.ClientIP()))
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

		// Check request age (prevent replay attacks)
		requestTime := time.Unix(timestamp, 0)
		if time.Since(requestTime) > MaxRequestAge {
			s.logger.Warn("Request timestamp too old",
				zap.String("service", serviceName),
				zap.Time("timestamp", requestTime),
				zap.Duration("age", time.Since(requestTime)))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "request too old"})
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
			c.Request.Body = io.NopCloser(io.Reader(body))
		}

		// Compute expected signature
		expectedSignature := s.computeSignature(c.Request.Method, c.Request.URL.Path, timestamp, body)

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

// computeSignature creates HMAC-SHA256 signature
// Format: HMAC-SHA256(secret, "method|path|timestamp|body_hash")
func (s *Signer) computeSignature(method, path string, timestamp int64, body []byte) string {
	// Hash body
	bodyHash := sha256.Sum256(body)

	// Create message to sign
	message := fmt.Sprintf("%s|%s|%d|%s",
		method,
		path,
		timestamp,
		hex.EncodeToString(bodyHash[:]))

	// Compute HMAC
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(message))

	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies a signature without using middleware
func (s *Signer) VerifySignature(method, path string, timestamp int64, body []byte, signature string) error {
	// Check timestamp
	requestTime := time.Unix(timestamp, 0)
	if time.Since(requestTime) > MaxRequestAge {
		return ErrRequestTooOld
	}

	// Compute expected signature
	expectedSignature := s.computeSignature(method, path, timestamp, body)

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

// NewSigningTransport creates a transport that signs all requests
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

// NewSigningClient creates an HTTP client that automatically signs all requests
func NewSigningClient(serviceName, secret string, logger *zap.Logger) *http.Client {
	signer := NewSigner(serviceName, secret, logger)
	return &http.Client{
		Transport: NewSigningTransport(nil, signer),
		Timeout:   30 * time.Second,
	}
}
