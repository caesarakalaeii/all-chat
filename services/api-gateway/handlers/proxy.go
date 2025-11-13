package handlers

import (
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/gin-gonic/gin"
)

// ProxyHandler handles proxying requests to backend services
type ProxyHandler struct {
	registry *models.ServiceRegistry
	client   *http.Client
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(registry *models.ServiceRegistry) *ProxyHandler {
	return &ProxyHandler{
		registry: registry,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects, let the client handle them
				return http.ErrUseLastResponse
			},
		},
	}
}

// ForwardRequest forwards the incoming request to the appropriate backend service
func (p *ProxyHandler) ForwardRequest(c *gin.Context) {
	// Get the full request path
	path := c.Request.URL.Path

	// Find the service that should handle this request
	service := p.registry.GetServiceForPath(path)
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "no service found for path: " + path,
		})
		return
	}

	// Build the backend URL
	backendURL := service.BaseURL + path
	if c.Request.URL.RawQuery != "" {
		backendURL += "?" + c.Request.URL.RawQuery
	}

	// Create new request to backend
	backendReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		c.Request.Method,
		backendURL,
		c.Request.Body,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create backend request",
		})
		return
	}

	// Copy headers from original request
	copyHeaders(backendReq.Header, c.Request.Header)

	// Forward request to backend
	backendResp, err := p.client.Do(backendReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "backend service unavailable: " + err.Error(),
		})
		return
	}
	defer backendResp.Body.Close()

	// Copy response headers from backend
	copyHeaders(c.Writer.Header(), backendResp.Header)

	// Set status code
	c.Status(backendResp.StatusCode)

	// Copy response body
	_, err = io.Copy(c.Writer, backendResp.Body)
	if err != nil {
		// Log error but don't send response as headers are already written
		c.Error(err)
	}
}

// copyHeaders copies HTTP headers from src to dst, excluding hop-by-hop headers
func copyHeaders(dst, src http.Header) {
	// Headers that should not be proxied (hop-by-hop headers)
	hopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}

	for key, values := range src {
		if !hopHeaders[key] {
			for _, value := range values {
				dst.Add(key, value)
			}
		}
	}
}
