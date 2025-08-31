package bodhveda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpClient struct {
	apiKey      string
	baseURL     string
	debug       bool
	innerClient *http.Client
}

func newHTTPClient(apiKey, baseURL string, debug bool) *httpClient {
	return &httpClient{
		apiKey:      apiKey,
		baseURL:     baseURL,
		debug:       debug,
		innerClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (client *httpClient) Do(ctx context.Context, method, path string, body any, out any) error {
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

	// Handle errors explicitly
	if resp.StatusCode >= 400 {
		var apiErr BodhvedaError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			// fallback: return raw response
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return &apiErr
	}

	// Success response
	if out != nil {
		// Define a temporary wrapper to extract "data"
		var wrapper struct {
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(respBody, &wrapper); err != nil {
			return fmt.Errorf("failed to decode wrapper response: %w", err)
		}

		if len(wrapper.Data) == 0 {
			return fmt.Errorf("missing data in success response")
		}

		// Now unmarshal the actual data into out
		if err := json.Unmarshal(wrapper.Data, out); err != nil {
			return fmt.Errorf("failed to decode data field: %w", err)
		}
	}

	return nil
}
