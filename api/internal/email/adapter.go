// Package email holds the medium adapter interface and its provider
// implementations. An adapter turns a normalized outbound email into a
// provider-specific API call (v1: Resend, BYO-provider) and normalizes the
// result back into a provider message id we can correlate webhooks against
// (Phase 5). The provider is selected via the `enum.EmailProvider` discriminator
// stored on the project's email settings.
package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// ErrWebhookSignatureInvalid is returned by an adapter's webhook verification
// when the inbound signature does not match the project's signing secret. The
// ingestion endpoint maps it to 401.
var ErrWebhookSignatureInvalid = errors.New("webhook signature verification failed")

// WebhookEventKind is a provider-agnostic classification of an inbound delivery
// event. Adapters normalize each provider's event vocabulary onto these so the
// status-transition logic stays uniform across providers (Phase 5).
type WebhookEventKind string

const (
	// WebhookEventUnknown means the event is not one we track — ignore it.
	WebhookEventUnknown    WebhookEventKind = ""
	WebhookEventSent       WebhookEventKind = "sent"       // provider accepted the message
	WebhookEventDelivered  WebhookEventKind = "delivered"  // reached the recipient's mail server
	WebhookEventBounced    WebhookEventKind = "bounced"    // hard/soft bounce (terminal)
	WebhookEventComplained WebhookEventKind = "complained" // marked as spam (terminal)
	WebhookEventOpened     WebhookEventKind = "opened"     // soft signal (see Apple MPP caveat)
	WebhookEventClicked    WebhookEventKind = "clicked"    // soft signal
)

// NormalizedEvent is a provider event reduced to what the delivery record needs:
// which delivery row it belongs to (ProviderMessageID), what happened (Kind),
// when (At), and the raw event JSON (appended to provider_response for audit).
type NormalizedEvent struct {
	// ProviderEventID is the provider's stable per-event id (Resend/Svix's
	// `svix-id`), identical across retries of the same event. Used as the
	// idempotency key to dedup replays (#8). May be empty if the provider does not
	// supply one, in which case dedup is skipped.
	ProviderEventID   string
	ProviderMessageID string
	Kind              WebhookEventKind
	At                time.Time
	Raw               json.RawMessage
}

// Message is a normalized outbound email, provider-agnostic. FromName /
// FromAddress come from the project's email settings; the rest come from the
// send call's `email` block + the resolved recipient contact address.
type Message struct {
	FromName    string
	FromAddress string
	To          string
	Subject     string
	HTML        string
	Text        string
	// Headers are extra provider headers to set on the outbound message. In v1
	// this carries the RFC 8058 unsubscribe headers (List-Unsubscribe /
	// List-Unsubscribe-Post — Phase 6). The adapter passes them through to the
	// provider's headers map.
	Headers map[string]string
	// IdempotencyKey, when non-empty, makes the send idempotent at the provider:
	// a retry (Asynq re-runs the email:delivery task on transient failure) that
	// carries the same key will not deliver a duplicate email even if the first
	// attempt actually reached the provider before erroring. Callers pass a stable
	// per-delivery key (the notification_delivery row id).
	IdempotencyKey string
}

// SendResult is the normalized outcome of a provider send. ProviderMessageID is
// the id the provider assigns (used to match inbound webhooks in Phase 5).
type SendResult struct {
	Provider          enum.EmailProvider
	ProviderMessageID string
}

// Adapter sends a normalized Message via a specific provider and normalizes that
// provider's inbound delivery webhooks (Phase 5). Keeping both send and webhook
// normalization behind one interface is what lets a new provider (Postmark,
// managed-SES, …) slot in without touching the ingestion endpoint.
type Adapter interface {
	// Provider reports which provider this adapter targets.
	Provider() enum.EmailProvider
	// Send dispatches the message. A non-nil error means the send failed (the
	// caller records a failed delivery); a nil error returns the provider id.
	Send(ctx context.Context, msg Message) (SendResult, error)

	// VerifyWebhookSignature checks the raw inbound request against the project's
	// webhook signing secret. It returns ErrWebhookSignatureInvalid when the
	// signature does not match (the endpoint maps that to 401). The signing secret
	// is provider-specific (Resend uses Svix `whsec_...`) and is passed per-call —
	// it is NOT the send API key.
	VerifyWebhookSignature(secret string, headers http.Header, body []byte) error

	// NormalizeWebhookEvent parses a (verified) provider event into a
	// provider-agnostic NormalizedEvent. A Kind of WebhookEventUnknown means the
	// event is not one we track and should be ignored.
	NormalizeWebhookEvent(headers http.Header, body []byte) (NormalizedEvent, error)
}

// NewAdapter builds the adapter for a provider, given the project's decrypted
// provider secret (API key). Only Resend is wired in v1; the discriminator makes
// adding Postmark/Mailgun/managed-SES a matter of adding a case here.
//
// The webhook path (which only calls VerifyWebhookSignature / NormalizeWebhookEvent,
// never Send) may pass an empty apiKey — those methods take the webhook signing
// secret separately.
func NewAdapter(provider enum.EmailProvider, apiKey string) (Adapter, error) {
	switch provider {
	case enum.EmailProviderResend:
		return NewResendAdapter(apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported email provider: %q", provider)
	}
}
