package nodeutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	url string
}

func NewClient(host string) *Client {
	return &Client{url: fmt.Sprintf("http://%s:8000", host)}
}

func (c *Client) GetDataSize() (int64, error) {
	response, err := http.Get(c.url + "/data_size")
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s", string(body))
	}

	return strconv.ParseInt(string(body), 10, 64)
}

func (c *Client) GetLatestHeight() (int64, error) {
	response, err := http.Get(c.url + "/latest_height")
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s", string(body))
	}

	return strconv.ParseInt(string(body), 10, 64)
}

func (c *Client) RequiresUpgrade() (bool, error) {
	response, err := http.Get(c.url + "/must_upgrade")
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return false, err
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusUpgradeRequired {
		return false, fmt.Errorf("%s", string(body))
	}

	return strconv.ParseBool(string(body))
}

func (c *Client) ShutdownNodeUtilsServer() error {
	response, err := http.Post(c.url+"/shutdown", "text/plain", nil)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

func (c *Client) ListSnapshots() ([]int64, error) {
	response, err := http.Get(c.url + "/snapshots")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var snapshots []int64
	err = json.Unmarshal(body, &snapshots)
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (c *Client) GetCPUStats(since time.Duration) (float64, error) {
	endpoint := c.url + "/stats/cpu"
	if since > 0 {
		params := url.Values{}
		params.Set("average", since.String())
		endpoint += "?" + params.Encode()
	}

	resp, err := http.Get(endpoint)
	if err != nil {
		return 0, fmt.Errorf("http get error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, b)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read error: %w", err)
	}

	val, err := strconv.ParseFloat(string(body), 64)
	if err != nil {
		return 0, fmt.Errorf("parse float error: %w", err)
	}

	return val, nil
}

func (c *Client) GetMemoryStats(since time.Duration) (uint64, error) {
	endpoint := c.url + "/stats/memory"
	if since > 0 {
		params := url.Values{}
		params.Set("average", since.String())
		endpoint += "?" + params.Encode()
	}

	resp, err := http.Get(endpoint)
	if err != nil {
		return 0, fmt.Errorf("http get error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, b)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read error: %w", err)
	}

	val, err := strconv.ParseUint(string(body), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse uint error: %w", err)
	}

	return val, nil
}
