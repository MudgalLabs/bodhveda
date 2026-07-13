package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

const resendSendURL = "https://api.resend.com/emails"

// ResendAdapter sends email via the Resend HTTP API. We call the REST endpoint
// directly (no Resend Go SDK dependency) — the request is a single JSON POST.
type ResendAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewResendAdapter(apiKey string) *ResendAdapter {
	return &ResendAdapter{
		apiKey:  apiKey,
		baseURL: resendSendURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *ResendAdapter) Provider() enum.EmailProvider {
	return enum.EmailProviderResend
}

type resendSendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

type resendSendResponse struct {
	ID string `json:"id"`
}

type resendErrorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (a *ResendAdapter) Send(ctx context.Context, msg Message) (SendResult, error) {
	// Resend expects "Name <address>" for a named from-identity.
	from := msg.FromAddress
	if msg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", msg.FromName, msg.FromAddress)
	}

	body, err := json.Marshal(resendSendRequest{
		From:    from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		HTML:    msg.HTML,
		Text:    msg.Text,
	})
	if err != nil {
		return SendResult{}, fmt.Errorf("marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL, bytes.NewReader(body))
	if err != nil {
		return SendResult{}, fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return SendResult{}, fmt.Errorf("resend request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr resendErrorResponse
		_ = json.Unmarshal(respBody, &apiErr)
		if apiErr.Message != "" {
			return SendResult{}, fmt.Errorf("resend send failed (%d): %s: %s", resp.StatusCode, apiErr.Name, apiErr.Message)
		}
		return SendResult{}, fmt.Errorf("resend send failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var parsed resendSendResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return SendResult{}, fmt.Errorf("decode resend response: %w", err)
	}

	return SendResult{
		Provider:          enum.EmailProviderResend,
		ProviderMessageID: parsed.ID,
	}, nil
}
