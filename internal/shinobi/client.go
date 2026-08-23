package shinobi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a pure-Go HTTP client for the Shinobi NVR REST API.
type Client struct {
	apiURL   string
	apiKey   string
	groupKey string
	http     *http.Client
}

// NewClient initializes a new Shinobi API client with clean URL and default timeout.
func NewClient(apiURL, apiKey, groupKey string) *Client {
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	return &Client{
		apiURL:   apiURL,
		apiKey:   strings.TrimSpace(apiKey),
		groupKey: strings.TrimSpace(groupKey),
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// WithHTTPClient returns a copy of the client with a custom http.Client.
func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	return &Client{
		apiURL:   c.apiURL,
		apiKey:   c.apiKey,
		groupKey: c.groupKey,
		http:     httpClient,
	}
}

// APIURL returns the configured Shinobi base API URL.
func (c *Client) APIURL() string { return c.apiURL }

// GroupKey returns the configured Shinobi Group Key (ke).
func (c *Client) GroupKey() string { return c.groupKey }

// ListMonitors queries all monitors under the configured Group Key.
// GET /:apiKey/monitor/:groupKey
func (c *Client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return nil, fmt.Errorf("shinobi client not fully configured")
	}

	endpoint := fmt.Sprintf("%s/%s/monitor/%s", c.apiURL, c.apiKey, c.groupKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build list monitors request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute list monitors request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read list monitors response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list monitors failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Unmarshal monitors array (or object wrapped array).
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		var mons []Monitor
		if err := json.Unmarshal(body, &mons); err != nil {
			return nil, fmt.Errorf("unmarshal monitors array: %w", err)
		}
		return mons, nil
	}

	// Check if Shinobi returned an error object e.g. {"ok":false,"msg":"..."}
	var errResp struct {
		OK  *bool  `json:"ok"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.OK != nil && !*errResp.OK {
		return nil, fmt.Errorf("shinobi error: %s", errResp.Msg)
	}

	// Try object wrapping formats: {"monitors": [...]} or {"data": [...]}
	var wrap struct {
		Monitors []Monitor `json:"monitors"`
		Data     []Monitor `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("parse monitors response: %w", err)
	}
	if len(wrap.Monitors) > 0 {
		return wrap.Monitors, nil
	}
	return wrap.Data, nil
}

// GetMonitor queries a single monitor by its monitor ID (mid).
// GET /:apiKey/monitor/:groupKey/:mid
func (c *Client) GetMonitor(ctx context.Context, mid string) (*Monitor, error) {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return nil, fmt.Errorf("shinobi client not fully configured")
	}
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return nil, fmt.Errorf("monitor ID (mid) is required")
	}

	endpoint := fmt.Sprintf("%s/%s/monitor/%s/%s", c.apiURL, c.apiKey, c.groupKey, mid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build get monitor request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get monitor request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read get monitor response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get monitor failed (status %d): %s", resp.StatusCode, string(body))
	}

	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		var mons []Monitor
		if err := json.Unmarshal(body, &mons); err != nil {
			return nil, fmt.Errorf("unmarshal monitor array: %w", err)
		}
		if len(mons) == 0 {
			return nil, fmt.Errorf("monitor %s not found", mid)
		}
		return &mons[0], nil
	}

	var mon Monitor
	if err := json.Unmarshal(body, &mon); err != nil {
		return nil, fmt.Errorf("unmarshal monitor object: %w", err)
	}
	if mon.Mid == "" {
		return nil, fmt.Errorf("monitor %s not found", mid)
	}
	return &mon, nil
}

// AddMonitor creates a new monitor on Shinobi.
// POST /:apiKey/configureMonitor/:groupKey/:mid
func (c *Client) AddMonitor(ctx context.Context, mon MonitorConfig) error {
	return c.saveMonitor(ctx, mon.Mid, mon)
}

// EditMonitor modifies an existing monitor on Shinobi.
// POST /:apiKey/configureMonitor/:groupKey/:mid
func (c *Client) EditMonitor(ctx context.Context, mid string, mon MonitorConfig) error {
	return c.saveMonitor(ctx, mid, mon)
}

func (c *Client) saveMonitor(ctx context.Context, mid string, mon MonitorConfig) error {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return fmt.Errorf("shinobi client not fully configured")
	}
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return fmt.Errorf("monitor ID (mid) is required")
	}
	mon.Mid = mid
	if mon.Ke == "" {
		mon.Ke = c.groupKey
	}

	monJSON, err := json.Marshal(mon)
	if err != nil {
		return fmt.Errorf("marshal monitor config: %w", err)
	}

	formData := url.Values{}
	formData.Set("data", string(monJSON))

	endpoint := fmt.Sprintf("%s/%s/configureMonitor/%s/%s", c.apiURL, c.apiKey, c.groupKey, mid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("build save monitor request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute save monitor request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read save monitor response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("save monitor failed (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		OK  *bool  `json:"ok"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(body, &res) == nil && res.OK != nil && !*res.OK {
		return fmt.Errorf("save monitor error: %s", res.Msg)
	}

	return nil
}

// DeleteMonitor deletes a monitor from Shinobi.
// GET /:apiKey/monitor/:groupKey/:mid/delete
func (c *Client) DeleteMonitor(ctx context.Context, mid string) error {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return fmt.Errorf("shinobi client not fully configured")
	}
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return fmt.Errorf("monitor ID (mid) is required")
	}

	endpoint := fmt.Sprintf("%s/%s/configureMonitor/%s/%s/delete", c.apiURL, c.apiKey, c.groupKey, mid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build delete monitor request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute delete monitor request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read delete monitor response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete monitor failed (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		OK  *bool  `json:"ok"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(body, &res) == nil && res.OK != nil && !*res.OK {
		return fmt.Errorf("delete monitor error: %s", res.Msg)
	}

	return nil
}

// ChangeMonitorState controls the stream/recording state of a monitor.
// Valid states: "start", "stop", "record", "idle", "restart".
// GET /:apiKey/monitor/:groupKey/:mid/:state
func (c *Client) ChangeMonitorState(ctx context.Context, mid, state string) error {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return fmt.Errorf("shinobi client not fully configured")
	}
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return fmt.Errorf("monitor ID (mid) is required")
	}
	state = strings.ToLower(strings.TrimSpace(state))
	switch state {
	case "start", "stop", "record", "idle", "restart":
		// valid
	default:
		return fmt.Errorf("invalid monitor state %q (must be start, stop, record, idle, or restart)", state)
	}

	endpoint := fmt.Sprintf("%s/%s/monitor/%s/%s/%s", c.apiURL, c.apiKey, c.groupKey, mid, state)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build change monitor state request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute change monitor state request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read change monitor state response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("change monitor state failed (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		OK  *bool  `json:"ok"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(body, &res) == nil && res.OK != nil && !*res.OK {
		return fmt.Errorf("change monitor state error: %s", res.Msg)
	}

	return nil
}

// GetVideos retrieves the list of recorded video files for a given monitor.
// GET /:apiKey/videos/:groupKey/:mid?limit=:limit
func (c *Client) GetVideos(ctx context.Context, mid string, limit int) ([]Video, error) {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return nil, fmt.Errorf("shinobi client not fully configured")
	}
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return nil, fmt.Errorf("monitor ID (mid) is required")
	}
	if limit <= 0 {
		limit = 50
	}

	endpoint := fmt.Sprintf("%s/%s/videos/%s/%s?limit=%d", c.apiURL, c.apiKey, c.groupKey, mid, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build get videos request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get videos request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read get videos response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get videos failed (status %d): %s", resp.StatusCode, string(body))
	}

	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		var videos []Video
		if err := json.Unmarshal(body, &videos); err != nil {
			return nil, fmt.Errorf("unmarshal videos array: %w", err)
		}
		return videos, nil
	}

	var wrap struct {
		Videos []Video `json:"videos"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("parse videos response: %w", err)
	}
	return wrap.Videos, nil
}

// Status checks the Shinobi API connectivity and returns system status.
func (c *Client) Status(ctx context.Context) (*ShinobiStatus, error) {
	if c.apiURL == "" || c.apiKey == "" || c.groupKey == "" {
		return &ShinobiStatus{
			Configured: false,
			Connected:  false,
			APIURL:     c.apiURL,
			GroupKey:   c.groupKey,
		}, nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	mons, err := c.ListMonitors(checkCtx)
	if err != nil {
		return &ShinobiStatus{
			Configured:   true,
			Connected:    false,
			APIURL:       c.apiURL,
			GroupKey:     c.groupKey,
			MonitorCount: 0,
			Error:        err.Error(),
		}, nil
	}

	return &ShinobiStatus{
		Configured:   true,
		Connected:    true,
		APIURL:       c.apiURL,
		GroupKey:     c.groupKey,
		MonitorCount: len(mons),
	}, nil
}
