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
	return strconv.ParseInt(string(body), 10, 64)
}
