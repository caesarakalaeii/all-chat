package innertube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"go.uber.org/zap"
)

var (
	reClientVersion = regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION":"([^"]+)"`)
	reAPIKey        = regexp.MustCompile(`"INNERTUBE_API_KEY":"([^"]+)"`)
)

// ClientConfig holds dynamically-fetched InnerTube client configuration.
type ClientConfig struct {
	APIKey        string
	ClientVersion string
}

// FetchClientConfig fetches the current InnerTube API key and client version
// from the YouTube homepage. Falls back to the compiled-in defaults on any error
// so a network hiccup at startup never prevents the service from starting.
func FetchClientConfig(ctx context.Context, logger *zap.Logger) ClientConfig {
	cfg := ClientConfig{
		APIKey:        DefaultAPIKey,
		ClientVersion: DefaultClientVersion,
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, "https://www.youtube.com", nil)
	if err != nil {
		logger.Warn("Failed to build YouTube homepage request, using hardcoded InnerTube config",
			zap.Error(err))
		return cfg
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("Failed to fetch YouTube homepage, using hardcoded InnerTube config",
			zap.Error(err))
		return cfg
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("Failed to read YouTube homepage body, using hardcoded InnerTube config",
			zap.Error(err))
		return cfg
	}

	if m := reClientVersion.FindSubmatch(body); len(m) == 2 {
		cfg.ClientVersion = string(m[1])
	} else {
		logger.Warn("INNERTUBE_CLIENT_VERSION not found in YouTube homepage, using hardcoded value",
			zap.String("fallback", cfg.ClientVersion))
	}

	if m := reAPIKey.FindSubmatch(body); len(m) == 2 {
		cfg.APIKey = string(m[1])
	} else {
		logger.Warn("INNERTUBE_API_KEY not found in YouTube homepage, using hardcoded value",
			zap.String("fallback", fmt.Sprintf("%.8s...", cfg.APIKey)))
	}

	logger.Info("Fetched InnerTube client config from YouTube",
		zap.String("client_version", cfg.ClientVersion),
		zap.String("api_key_prefix", fmt.Sprintf("%.8s...", cfg.APIKey)),
	)

	return cfg
}
