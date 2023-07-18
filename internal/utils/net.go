package utils

import (
	"encoding/json"
	"io"
	"net/http"
)

func FetchJson(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func GetGenesisFromNodeRPC(url string) (string, error) {

	out := struct {
		Result struct {
			Genesis json.RawMessage `json:"genesis"`
		} `json:"result"`
	}{}
	genesis := ""

	res, err := http.Get(url)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(out.Result.Genesis, "", "  ")
	if err != nil {
		return "", err
	}

	genesis = string(b) + "\n"

	return genesis, nil
}
