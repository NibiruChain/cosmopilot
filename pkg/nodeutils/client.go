package nodeutils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	// httpClient is a shared HTTP client with reasonable timeout
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}
)

// Client provides methods to interact with the node-utils HTTP server.
type Client struct {
	url string
}

// NewClient creates a new node-utils client for the given host.
// The host should be a hostname or IP address without scheme or port.
func NewClient(host string) *Client {
	return &Client{url: fmt.Sprintf("http://%s:8000", host)}
}

// httpGet performs an HTTP GET request and returns the response body as a string.
// It handles error checking and ensures the response body is properly closed.
func (c *Client) httpGet(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

// httpGetJSON performs an HTTP GET request and unmarshals the JSON response.
func (c *Client) httpGetJSON(ctx context.Context, endpoint string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.Unmarshal(body, target)
}

// GetDataSize returns the current size of the node's data directory in bytes.
func (c *Client) GetDataSize(ctx context.Context) (int64, error) {
	body, err := c.httpGet(ctx, "/data_size")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(body, 10, 64)
}

// GetLatestHeight returns the latest block height from the blockchain node.
func (c *Client) GetLatestHeight(ctx context.Context) (int64, error) {
	body, err := c.httpGet(ctx, "/latest_height")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(body, 10, 64)
}

// RequiresUpgrade checks if the node requires an upgrade.
// Returns true if an upgrade is required, false otherwise.
func (c *Client) RequiresUpgrade(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/must_upgrade", nil)
	if err != nil {
		return false, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUpgradeRequired {
		return false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return strconv.ParseBool(string(body))
}

// ShutdownNodeUtilsServer sends a shutdown signal to the node-utils server.
func (c *Client) ShutdownNodeUtilsServer(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/shutdown", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// ListSnapshots returns a list of available snapshot heights.
func (c *Client) ListSnapshots(ctx context.Context) ([]int64, error) {
	var snapshots []int64
	if err := c.httpGetJSON(ctx, "/snapshots", &snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

// GetCPUStats returns the CPU usage percentage.
// If since is greater than 0, it returns the average over that duration.
func (c *Client) GetCPUStats(ctx context.Context, since time.Duration) (float64, error) {
	endpoint := "/stats/cpu"
	if since > 0 {
		params := url.Values{}
		params.Set("average", since.String())
		endpoint += "?" + params.Encode()
	}

	body, err := c.httpGet(ctx, endpoint)
	if err != nil {
		return 0, fmt.Errorf("failed to get CPU stats: %w", err)
	}

	val, err := strconv.ParseFloat(body, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU stats: %w", err)
	}

	return val, nil
}

// GetMemoryStats returns the memory usage in bytes.
// If since is greater than 0, it returns the average over that duration.
func (c *Client) GetMemoryStats(ctx context.Context, since time.Duration) (uint64, error) {
	endpoint := "/stats/memory"
	if since > 0 {
		params := url.Values{}
		params.Set("average", since.String())
		endpoint += "?" + params.Encode()
	}

	body, err := c.httpGet(ctx, endpoint)
	if err != nil {
		return 0, fmt.Errorf("failed to get memory stats: %w", err)
	}

	val, err := strconv.ParseUint(body, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory stats: %w", err)
	}

	return val, nil
}

// IsStateSyncing returns true if the node is currently performing state-sync.
func (c *Client) IsStateSyncing(ctx context.Context) (bool, error) {
	body, err := c.httpGet(ctx, "/state_syncing")
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(body)
}
