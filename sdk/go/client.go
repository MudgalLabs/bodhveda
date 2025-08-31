package bodhveda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return err
		}

		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.apiKey)

	if client.debug {
		log.Printf("[Bodhveda DEBUG] Request: %s %s\n", method, client.baseURL+path)

		for k, v := range req.Header {
			if k == "Authorization" {
				log.Printf("[Bodhveda DEBUG] Request Header: %s: %s\n", k, "[REDACTED]")
			} else {
				log.Printf("[Bodhveda DEBUG] Request Header: %s: %v\n", k, v)
			}
		}

		if client.debug {
			logBodyTruncated("Request", reqBody)
		}
	}

	start := time.Now()

	resp, err := client.innerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	if client.debug {
		fmt.Printf("[Bodhveda DEBUG] Request Duration: %s\n", duration)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if client.debug {
		log.Printf("[Bodhveda DEBUG] Response Status: %s\n", resp.Status)
		logBodyTruncated("Request", respBody)
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

		if client.debug {
			logBodyTruncated("Response", wrapper.Data)
		}
	}

	return nil
}

const maxLogSize = 1000

func logBodyTruncated(prefix string, content []byte) {
	bodyToLog := string(content)

	if len(bodyToLog) > maxLogSize {
		bodyToLog = bodyToLog[:maxLogSize] + "...[truncated]"
	}

	log.Printf("[Bodhveda DEBUG] %s Body: %s\n", prefix, bodyToLog)
}
