package email

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

// --- Webhooks (Phase 5) ---
//
// Resend signs webhooks via Svix. The signature scheme:
//   - Headers: `svix-id`, `svix-timestamp`, `svix-signature`.
//   - Signing secret is `whsec_<base64>`; the key is base64-decode(<base64>).
//   - signedContent = "{svix-id}.{svix-timestamp}.{rawBody}".
//   - expected = base64(HMAC-SHA256(key, signedContent)).
//   - `svix-signature` is a space-separated list of "v1,<sig>" entries; the
//     request is valid if any entry's signature matches (constant-time compare).
// We verify manually rather than pull in the Svix SDK (consistent with the
// no-Resend-SDK decision in Phase 4).

const svixSecretPrefix = "whsec_"

// webhookToleranceSeconds bounds how far the svix-timestamp may drift from now,
// to blunt replay of captured events. Svix's own default is 5 minutes.
const webhookToleranceSeconds = 5 * 60

func (a *ResendAdapter) VerifyWebhookSignature(secret string, headers http.Header, body []byte) error {
	msgID := headers.Get("svix-id")
	timestamp := headers.Get("svix-timestamp")
	sigHeader := headers.Get("svix-signature")
	if msgID == "" || timestamp == "" || sigHeader == "" {
		return ErrWebhookSignatureInvalid
	}

	// Reject stale/future timestamps (replay protection).
	ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return ErrWebhookSignatureInvalid
	}
	now := time.Now().Unix()
	if ts < now-webhookToleranceSeconds || ts > now+webhookToleranceSeconds {
		return ErrWebhookSignatureInvalid
	}

	// Derive the raw HMAC key from the whsec_ secret.
	rawSecret := strings.TrimPrefix(strings.TrimSpace(secret), svixSecretPrefix)
	key, err := base64.StdEncoding.DecodeString(rawSecret)
	if err != nil || len(key) == 0 {
		return ErrWebhookSignatureInvalid
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msgID + "." + timestamp + "."))
	mac.Write(body)
	expected := mac.Sum(nil)

	// The header carries one or more space-delimited "version,signature" pairs.
	for _, part := range strings.Split(sigHeader, " ") {
		_, sig, ok := strings.Cut(part, ",")
		if !ok {
			continue
		}
		got, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			continue
		}
		if hmac.Equal(got, expected) {
			return nil
		}
	}

	return ErrWebhookSignatureInvalid
}

// resendWebhookEvent is the Resend/Svix webhook envelope. `type` is e.g.
// "email.delivered"; `data.email_id` is the id we set on send as the delivery
// row's provider_message_id.
type resendWebhookEvent struct {
	Type      string          `json:"type"`
	CreatedAt string          `json:"created_at"`
	Data      resendEventData `json:"data"`
}

type resendEventData struct {
	EmailID   string `json:"email_id"`
	CreatedAt string `json:"created_at"`
}

func (a *ResendAdapter) NormalizeWebhookEvent(_ http.Header, body []byte) (NormalizedEvent, error) {
	var ev resendWebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return NormalizedEvent{}, fmt.Errorf("decode resend webhook: %w", err)
	}

	kind := resendEventKind(ev.Type)
	if kind == WebhookEventUnknown {
		return NormalizedEvent{Kind: WebhookEventUnknown, Raw: json.RawMessage(body)}, nil
	}

	// Prefer the event timestamp; fall back to now if absent/unparseable.
	at := time.Now().UTC()
	for _, candidate := range []string{ev.CreatedAt, ev.Data.CreatedAt} {
		if candidate == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, candidate); err == nil {
			at = parsed.UTC()
			break
		}
	}

	return NormalizedEvent{
		ProviderMessageID: ev.Data.EmailID,
		Kind:              kind,
		At:                at,
		Raw:               json.RawMessage(body),
	}, nil
}

// resendEventKind maps Resend event `type` strings onto our provider-agnostic
// kinds. Unmapped types (e.g. email.delivery_delayed) fall through to unknown and
// are ignored.
func resendEventKind(t string) WebhookEventKind {
	switch t {
	case "email.sent":
		return WebhookEventSent
	case "email.delivered":
		return WebhookEventDelivered
	case "email.bounced":
		return WebhookEventBounced
	case "email.complained":
		return WebhookEventComplained
	case "email.opened":
		return WebhookEventOpened
	case "email.clicked":
		return WebhookEventClicked
	default:
		return WebhookEventUnknown
	}
}
