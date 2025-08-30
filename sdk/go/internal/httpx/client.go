// Package httpx provides a simple HTTP client for interacting with the Bodhveda API.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey      string
	baseURL     string
	innerClient *http.Client
}

func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:      apiKey,
		baseURL:     baseURL,
		innerClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (client *Client) Do(method, path string, body any, out any) error {
	var bodyReader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, client.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.apiKey)

	resp, err := client.innerClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return errors.New(string(respBody))
	}

	if out != nil {
		return json.Unmarshal(respBody, out)
	}

	return nil
}
