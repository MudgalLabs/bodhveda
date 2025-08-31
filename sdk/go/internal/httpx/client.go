// Package httpx provides a simple HTTP client for interacting with the Bodhveda API.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey      string
	baseURL     string
	debug       bool
	innerClient *http.Client
}

func NewClient(apiKey, baseURL string, debug bool) *Client {
	return &Client{
		apiKey:      apiKey,
		baseURL:     baseURL,
		debug:       debug,
		innerClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (client *Client) Do(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
		if client.debug {
			fmt.Printf("[DEBUG] Request Body: %s\n", string(data))
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.apiKey)

	if client.debug {
		fmt.Printf("[DEBUG] Request: %s %s\n", method, client.baseURL+path)
		for k, v := range req.Header {
			fmt.Printf("[DEBUG] Request Header: %s: %v\n", k, v)
		}
	}

	resp, err := client.innerClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if client.debug {
		fmt.Printf("[DEBUG] Response Status: %s\n", resp.Status)
		fmt.Printf("[DEBUG] Response Body: %s\n", string(respBody))
	}

	if resp.StatusCode >= 400 {
		return errors.New(string(respBody))
	}

	if out != nil {
		return json.Unmarshal(respBody, out)
	}

	return nil
}
