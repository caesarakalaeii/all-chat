package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ServiceProxy handles proxying requests to backend services
type ServiceProxy struct {
	authProxy    *httputil.ReverseProxy
	overlayProxy *httputil.ReverseProxy
	emoteProxy   *httputil.ReverseProxy
	logger       *zap.Logger
}

// NewServiceProxy creates a new service proxy
func NewServiceProxy(authURL, overlayURL, emoteURL string, logger *zap.Logger) (*ServiceProxy, error) {
	authTarget, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}

	overlayTarget, err := url.Parse(overlayURL)
	if err != nil {
		return nil, err
	}

	emoteTarget, err := url.Parse(emoteURL)
	if err != nil {
		return nil, err
	}

	return &ServiceProxy{
		authProxy:    httputil.NewSingleHostReverseProxy(authTarget),
		overlayProxy: httputil.NewSingleHostReverseProxy(overlayTarget),
		emoteProxy:   httputil.NewSingleHostReverseProxy(emoteTarget),
		logger:       logger,
	}, nil
}

// ProxyToAuthService forwards requests to the auth service
func (p *ServiceProxy) ProxyToAuthService(c *gin.Context) {
	p.logger.Debug("Proxying to auth service",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	p.authProxy.ServeHTTP(c.Writer, c.Request)
}

// ProxyToOverlayManager forwards requests to the overlay manager
func (p *ServiceProxy) ProxyToOverlayManager(c *gin.Context) {
	p.logger.Debug("Proxying to overlay manager",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	p.overlayProxy.ServeHTTP(c.Writer, c.Request)
}

// ProxyToEmoteService forwards requests to the emote service
func (p *ServiceProxy) ProxyToEmoteService(c *gin.Context) {
	p.logger.Debug("Proxying to emote service",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	p.emoteProxy.ServeHTTP(c.Writer, c.Request)
}

// HealthCheck returns a simple health check response
func (p *ServiceProxy) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
