package sourcemanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

var (
	// ErrMissingSecret indicates a signing secret was not provided.
	ErrMissingSecret = errors.New("source manager signing secret is required")

	// ErrUnauthorized indicates the Source Manager rejected the token.
	ErrUnauthorized = errors.New("source manager authorization failed")

	// ErrLeadershipLost indicates the remote server reported leadership loss.
	ErrLeadershipLost = errors.New("leadership lost")
)

// LeadershipClient describes the subset of client functionality used for leadership.
type LeadershipClient interface {
	ClaimLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error)
	RenewLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error)
	ReleaseLeadership(ctx context.Context, platform, streamID, callerID string) error
	RegisterPeer(ctx context.Context, platform, callerID string) (peerCount int, err error)
}

// Client talks to the Source Manager HTTP API.
type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	tokenSource TokenSource
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// NewClient creates a new Source Manager API client.
func NewClient(rawURL string, tokenSource TokenSource, opts ...ClientOption) (*Client, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid source manager URL: %w", err)
	}

	client := &Client{
		baseURL:     parsed,
		tokenSource: tokenSource,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// GetSources fetches all active sources for the provided platform (empty returns all).
func (c *Client) GetSources(ctx context.Context, platform string) ([]*ActiveSource, error) {
	query := url.Values{}
	if platform != "" {
		query.Set("platform", platform)
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/sources", query, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.decodeError(resp)
	}

	var payload struct {
		Sources []*ActiveSource `json:"sources"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode sources response: %w", err)
	}

	return payload.Sources, nil
}

// ClaimLeadership attempts to become leader for the given stream ID.
func (c *Client) ClaimLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error) {
	reqBody := map[string]string{
		"platform":  platform,
		"stream_id": streamID,
		"caller_id": callerID,
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/leadership/claim", nil, reqBody)
	if err != nil {
		return false, err
	}

	resp, err := c.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return false, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return false, c.decodeError(resp)
	}

	var payload ClaimResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("failed to decode claim response: %w", err)
	}

	return payload.Acquired, nil
}

// RenewLeadership refreshes an existing leadership claim.
func (c *Client) RenewLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error) {
	reqBody := map[string]string{
		"platform":  platform,
		"stream_id": streamID,
		"caller_id": callerID,
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/leadership/renew", nil, reqBody)
	if err != nil {
		return false, err
	}

	resp, err := c.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return false, ErrUnauthorized
	}
	if resp.StatusCode == http.StatusGone {
		return false, ErrLeadershipLost
	}
	if resp.StatusCode != http.StatusOK {
		return false, c.decodeError(resp)
	}

	var payload RenewResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("failed to decode renew response: %w", err)
	}

	return payload.Renewed, nil
}

// ReleaseLeadership releases a leadership claim.
func (c *Client) ReleaseLeadership(ctx context.Context, platform, streamID, callerID string) error {
	reqBody := map[string]string{
		"platform":  platform,
		"stream_id": streamID,
		"caller_id": callerID,
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/leadership/release", nil, reqBody)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return c.decodeError(resp)
	}

	return nil
}

// RegisterPeer registers this instance as an active peer for the given platform
// and returns the total number of active peers for that platform.
func (c *Client) RegisterPeer(ctx context.Context, platform, callerID string) (int, error) {
	reqBody := map[string]string{
		"platform":  platform,
		"caller_id": callerID,
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/leadership/peers/register", nil, reqBody)
	if err != nil {
		return 0, err
	}

	resp, err := c.do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return 0, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return 0, c.decodeError(resp)
	}

	var payload PeerResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("failed to decode peer response: %w", err)
	}

	return payload.PeerCount, nil
}

// ActivateSource marks a channel's source as active in the DB.
// Listeners call this when they start polling to prevent the cleanup job from
// deactivating the source due to staleness.
func (c *Client) ActivateSource(ctx context.Context, platform, channelID string) error {
	reqBody := map[string]string{
		"platform":   platform,
		"channel_id": channelID,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/sources/activate", nil, reqBody)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return c.decodeError(resp)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, query url.Values, body interface{}) (*http.Request, error) {
	if c.tokenSource == nil {
		return nil, fmt.Errorf("token source is required")
	}

	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, endpoint)
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		buf = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) decodeError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10)) // 4KB
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("source manager %s: %s", resp.Status, msg)
}
