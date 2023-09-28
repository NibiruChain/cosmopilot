package nodeutils

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
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
		return 0, fmt.Errorf(string(body))
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
		return 0, fmt.Errorf(string(body))
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
		return false, fmt.Errorf(string(body))
	}

	return strconv.ParseBool(string(body))
}
